---
status: accepted
---

# Revert CoreDNS AAAA suppression: root cause was the 1.14.6 autopath/prefetch bug, not missing IPv6 egress alone

On 2026-08-24, `flux-operator`'s periodic `ghcr.io` OCI artifact fetch started intermittently failing with `network is unreachable`, flapping `FluxInstance` to `ArtifactFailed`. The working theory at the time (commit `38ad2f75`): this cluster has no IPv6 route (Cilium's `ipv6Enabled` is off by chart default, nodes are IPv4-only per `talconfig.yaml`), and GitHub's CDN sometimes returns an IPv6 address for `ghcr.io`, so an occasional real AAAA answer was enough to trigger the failure. The fix was to force every AAAA query through a `template IN AAAA . { rcode NOERROR }` block in the Corefile, so CoreDNS would never answer AAAA at all. It worked — the flapping stopped.

It wasn't the actual root cause, only a symptom-suppressor. [onedr0p/home-ops#11619](https://github.com/onedr0p/home-ops/pull/11619) diagnosed the same class of failure and traced it to a real CoreDNS bug: 1.14.3+ (we were on the coredns chart's 1.47.0, which ships CoreDNS 1.14.6) refreshes cache entries during prefetch with a synthetic empty remote address, so the `autopath` plugin can't match the source pod, and the search-expanded key gets replaced by the `kubernetes` plugin's NXDOMAIN ahead of the still-valid positive entry — tracked upstream as [coredns/coredns#8390](https://github.com/coredns/coredns/issues/8390). The bug corrupts the **A** answer specifically; AAAA resolves normally. In an IPv4-only environment, a client left holding only an AAAA answer builds a v6-only dial list and fails with exactly the `network is unreachable` we saw. Our AAAA-suppression block "fixed" it the same way onedr0p's did before they removed it: by making a v6-only dial list impossible, it papered over the real bug rather than fixing it — and left every genuine AAAA lookup silently broken (`NOERROR`/empty instead of a correct answer or `NXDOMAIN`) as a side effect.

We reverted to match upstream's actual fix:

- `kubernetes/apps/kube-system/coredns/app/ocirepository.yaml`: chart `1.47.0 → 1.46.2` (CoreDNS `1.14.6 → 1.13.1`, below the bug).
- `kubernetes/apps/kube-system/coredns/app/helmrelease.yaml`: removed the `template IN AAAA .` block — no longer needed once the underlying bug is gone.
- `.renovaterc.json5`: added a `packageRules` hold (`enabled: false`) on `ghcr.io/coredns/charts/coredns`, pointing at `coredns/coredns#8390`, so Renovate can't silently re-bump us back into the bug the way it did originally (`1.46.2 → 1.47.0` on 2026-08-19).

## searchDomains

onedr0p's PR also cleared DHCP-supplied search domains (`ResolverConfig.searchDomains.domains: []`) because their gateway's DHCP `search internal` was getting copied into every pod's `resolv.conf` and read by `autopath`, multiplying upstream queries (49% of forwarded queries in their sample were redundant `*.internal.internal` expansions) and correspondingly multiplying how often the prefetch race got hit.

We checked whether the same applies here: `talosctl get resolvers` on all three nodes currently reports an **empty** search-domains list, because of an unrelated Talos bug on our version (`v1.13.9`) — [siderolabs/talos#13270](https://github.com/siderolabs/talos/issues/13270) — where the DHCP-supplied domain (option 15) isn't applied to `ResolverSpec` at all. So the amplification vector onedr0p hit doesn't currently exist on this cluster; our exposure to `coredns/coredns#8390` is via CoreDNS's own normal `cluster.local`/`svc.cluster.local`/`default.svc.cluster.local` search walk, not an extra DHCP domain on top of it.

No `ResolverConfig` change made. This is a latent risk, not a fixed one: if `siderolabs/talos#13270` is ever fixed upstream (by us upgrading Talos, or by the bug being backported), DHCP search domains would start flowing into node `resolv.conf` again and this cluster would need the same `ResolverConfig.searchDomains.domains: []` override onedr0p applied.

## Considered options

- **Keep the AAAA-suppression block and also downgrade CoreDNS** ("belt and suspenders"). Rejected: once the version bug that made AAAA answers dangerous is gone, blanket-suppressing AAAA has no remaining justification and actively breaks legitimate AAAA lookups (e.g. any future dual-stack service or external AAAA-only record) for no benefit.
- **Proactively add the `ResolverConfig` searchDomains override now**, even though nothing is broken today. Rejected for now: `talosctl get resolvers` confirms this cluster isn't currently exposed to the leak (see above); adding an override for a non-reproducing issue is speculative hardening against a Talos bug we're not even relying on. Revisit if `siderolabs/talos#13270` is fixed or before/after any Talos upgrade — re-run `talosctl get resolvers` and check `SEARCH DOMAINS` is still empty as expected.
- **Open a GitHub issue to track re-enabling the coredns bump.** Rejected: the Renovate Dashboard already surfaces held packages, and this ADR records the why/when-to-revisit; a separate issue would be redundant for a one-line follow-up.

## Consequences

- CoreDNS pinned to chart `1.46.2` (binary `1.13.1`) until `coredns/coredns#8390` is fixed upstream; Renovate is held on the chart (`.renovaterc.json5`) so it can't silently re-bump past this.
- Real AAAA answers are no longer suppressed cluster-wide. If ghcr.io (or any other upstream) hands back an IPv6-only or v6-preferred answer again, and something in the request path lacks solid Happy-Eyeballs-style fallback, `network is unreachable`-style flapping could recur — but now for a genuinely different reason (no IPv6 egress on an isolated AAAA answer) than what was actually happening before (the 1.14.6 bug producing a *corrupted* A answer alongside a *valid* AAAA one). Watch `flux get ks -A` / FluxInstance status after this change; there is no dedicated alert for this specific failure mode.
- Before re-enabling AAAA suppression or otherwise reaching for this pattern again: confirm via `coredns_autopath_success_total` vs. cache-miss rate (as onedr0p did) or similar direct evidence that AAAA answers themselves (not a DNS-layer bug) are the actual trigger, rather than assuming "no IPv6 route + any AAAA answer = broken."
- Related but distinct: [ADR 0004](0004-defer-proxygroup-ha-until-ipv6-egress-is-fixed.md) covers the same "no cluster IPv6 egress" underlying fact, but for a different mechanism (Tailscale's ProxyGroup VIP-services control-plane API call requiring IPv6). That gap is unrelated to CoreDNS and remains open independently of this ADR.
- If `siderolabs/talos#13270` is fixed (by a Talos upgrade or otherwise): re-check `talosctl get resolvers` for non-empty `SEARCH DOMAINS`, and if DHCP is supplying one, add a `ResolverConfig` document with `searchDomains.domains: []` (and explicit `nameservers`, per the PR's note that a `searchDomains`-only document with no `nameservers` drops all DNS config on 1.13.x-era Talos) to prevent the same query-multiplication onedr0p hit.
