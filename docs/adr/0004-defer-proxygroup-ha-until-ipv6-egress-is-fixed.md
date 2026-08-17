---
status: accepted
---

# Defer Tailscale ProxyGroup-backed Service HA until cluster IPv6 egress is fixed

Grafana, Jellyfin, and Channels are each exposed to the tailnet via a `LoadBalancer` Service (`loadBalancerClass: tailscale`), each backed by its own single dedicated proxy pod. We attempted to move them onto a shared `ingress-proxies` `ProxyGroup` (`type: ingress`, 2 replicas) for high availability, per Tailscale's documented `tailscale.com/proxy-group` annotation pattern — no client-facing change, same hostname/port, purely an HA upgrade.

The `ProxyGroup` itself provisioned cleanly (`ProxyGroupReady`, both replicas `Running`). The Service annotation applied without error. But a `kubectl exec` + BPF conntrack check on Grafana's own pod (`10.69.5.195:3000`) showed every real connection still arriving from the *old* dedicated proxy's pod IP, never from either `ingress-proxies` replica — the cutover had not actually happened at the traffic-serving layer, despite every visible signal (no reconciler errors, clean annotation, healthy pods) suggesting it had.

Deleting the old dedicated proxy's `StatefulSet` to force the question caused a real ~4 minute Grafana outage. That surfaced the actual error, previously silent:

```
error getting Tailscale Service "grafana": Get "https://controlplane.tailscale.com/api/v2/tailnet/-/vip-services/svc:grafana":
dial tcp [2606:b740:49::111]:443: connect: network is unreachable
```

ProxyGroup-backed Services are implemented via a newer Tailscale control-plane primitive ("VIP Services", `vip-services/svc:<name>`), and the operator reaches `controlplane.tailscale.com` for that API over IPv6 from inside the cluster. This cluster's pod network doesn't have working IPv6 egress. The legacy per-Service dedicated-proxy path (what all three services use today) apparently doesn't hit this API at all, which is why it's worked fine for 82+ days undisturbed.

Both services were reverted to their original dedicated-proxy annotations (no `tailscale.com/proxy-group`), restoring service. The `ingress-proxies` `ProxyGroup` resource is left in place — healthy, harmless, backing nothing — since re-deploying it later costs nothing once the real blocker is fixed. `dependsOn: tailscale-operator` was added to the Grafana/Jellyfin/Channels `Kustomization`s regardless (see git history); it's still correct hygiene independent of this gap; it just wasn't sufficient on its own.

## Considered options

- **Debug and fix IPv6 egress as part of this change.** Rejected for now: root-causing why cluster pods can't reach IPv6 destinations (missing route, disabled dual-stack on a CNI config, upstream ISP/WAN IPv6 not delegated to the cluster's egress path, etc.) is a separate, potentially much larger investigation than "add an annotation to three Services," and doing it live while Grafana was down was the wrong moment to start.
- **Keep retrying / wait for the operator's backoff to eventually succeed.** Rejected: this isn't a transient failure that resolves with patience — "no route to an IPv6 destination" doesn't self-heal, and leaving Services in a permanently-erroring state is worse than reverting.

## Consequences

- Grafana, Jellyfin, and Channels remain single-dedicated-proxy (no HA) until this is picked back up.
- Before retrying ProxyGroup HA on any Service: confirm cluster pod IPv6 egress works at all (e.g. `kubectl exec` into any pod and `curl -6 https://controlplane.tailscale.com` or similar) as a precondition, not an afterthought discovered mid-outage.
- If/when IPv6 egress is fixed, re-adding `tailscale.com/proxy-group: ingress-proxies` to each Service's annotations is the entire remaining change — the `ProxyGroup` and `dependsOn` wiring are already in place.
- Don't delete a Service's old dedicated proxy `StatefulSet` as part of a ProxyGroup migration without first confirming (via a conntrack-level check, not just "no errors in the logs") that the new identity is actually live and serving. "No errors" and "actually cut over" turned out to be different things here.
