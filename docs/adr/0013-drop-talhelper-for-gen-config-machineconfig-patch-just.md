---
status: accepted
---

# Drop talhelper for `talosctl gen config` + `machineconfig patch` + `just`

`talhelper` (`talos/talconfig.yaml` + `talos/patches/**`) is the layer that turns this repo's
declared machine config into what actually gets applied to `ser8a`/`ser8b`/`ser8c`. Two
independent facts converged to force this off talhelper, tracked from
[Migrate Talos machine config off talhelper to multi-doc kinds (#1504)](https://github.com/rwade628/homelab/issues/1504):

- **talhelper is dead upstream.** v3.1.17 is its last release — the maintainer archived the repo
  and pointed users at other tools. It's pinned to Talos machinery `v1.14.0-alpha.2`, which is
  missing 5 of the 9 multi-document Kubernetes config kinds Talos v1.14 GA ships (`KubeCoreDNSConfig`,
  `KubeletConfig`, `KubeNodeConfig`, `KubeClusterConfig`, `SecurityProfileConfig` — confirmed by
  diffing that alpha tag against the real `v1.14.0` tag; see
  [Research: Talos v1.14 GA multi-doc config kind schema (#1506)](https://github.com/rwade628/homelab/issues/1506)).
  There will be no future talhelper release to catch up.
- **It's already broken, today, against this repo's own config.** Running `talhelper genconfig`
  against the committed `talconfig.yaml` + patches at Talos v1.14.0 (the version already live on
  all 3 nodes) fails outright: Talos 1.14's default base config now auto-populates the multi-doc
  equivalents of `cluster.apiServer`/`controllerManager`/`scheduler`/`proxy`, and talhelper's
  legacy-field patches for those collide with them (`"kube-apiserver config is already set in
  v1alpha1 config"`, etc.), while the `admissionControl` delete-patch errors because that field no
  longer exists to delete. See
  [Task: Audit ser8a/ser8b/ser8c live config for drift vs repo-generated (#1507)](https://github.com/rwade628/homelab/issues/1507)
  for the full error output and per-field breakdown. This isn't a future-proofing exercise — the
  legacy-field path cannot regenerate a config today.

## Decision

Drop talhelper's three roles — base config authoring, per-node templating, and the `task talos:*`
apply commands — and replace each independently:

- **Base config**: generated fresh on demand via `talosctl gen config homelab <endpoint>
  --with-secrets <(sops -d talos/talsecret.sops.yaml)`, not hand-authored or committed. No secrets
  are regenerated — the same `talosctl gen secrets`-shaped bundle already sitting in
  `talos/talsecret.sops.yaml` (produced by talhelper, but format-identical to `gen secrets`'
  own output) is decrypted and fed straight in.
- **Machine config shape**: hand-written multi-document YAML patch files, layered onto the base
  with `talosctl machineconfig patch <base> -p @cluster.yaml -p @controlplane.yaml -p
  @nodes/<hostname>.yaml`, replacing `talos/patches/{global,controller}/*.yaml`. Full field→kind
  mapping in
  [Decide: target multi-doc structure (#1508)](https://github.com/rwade628/homelab/issues/1508);
  full file tree and contents in the companion spec,
  [`docs/talos-multidoc-migration-spec.md`](../talos-multidoc-migration-spec.md).
- **Per-node templating**: no templating engine — one literal, hand-written YAML file per node
  (`talos/nodes/<hostname>.yaml`), each holding only what's genuinely per-node: hostname, exact
  full-MAC link selector, and hardware-derived `KubeNodeConfig` labels. Everything else (bond,
  VLANs, VIP, disk selector, sysctls, etc.) is identical across all 3 nodes today and lives in the
  shared `cluster.yaml`.
- **Task runner**: `talos:*` moves out of `.taskfiles/talos/Taskfile.yaml` (currently broken —
  its `TALHELPER_CONFIG_FILE`/`TALHELPER_CLUSTER_DIR` vars resolve to a
  `kubernetes/bootstrap/talos/...` path that doesn't exist) into a `just` module
  (`talos/mod.just`, imported by a one-line root `.justfile`), giving `just talos <recipe>`.
  Scoped strictly to this subtree — not a repo-wide Taskfile→just migration. Full decision
  record: [Decide: talhelper replacement (#1509)](https://github.com/rwade628/homelab/issues/1509).

`onedr0p/home-ops` is the reference implementation this otherwise follows (worked example in
[Research #1505](https://github.com/rwade628/homelab/issues/1505)); the deviations from it are
each deliberate:

- **Secrets stay SOPS-in-repo** (`talos/talsecret.sops.yaml`), not externalized to 1Password —
  onedr0p's `op://` references need a 1Password Connect integration this repo doesn't have and has
  no other reason to add for this migration alone.
- **Exact per-node full-MAC `LinkAliasConfig` selectors**, not onedr0p's shared vendor-prefix
  match — onedr0p's trick only works because their fleet's per-node NIC-port count under that
  prefix is verified to disambiguate correctly; ours isn't, and a wrong shared match risks binding
  the bond to the wrong physical port on a node with more than one NIC from the same vendor.
- **Talos-native VIP kept** (`Layer2VIPConfig` on `bond0`, unchanged), not onedr0p's Cilium
  LoadBalancer + BGP-announced control-plane endpoint — replacing the actual Kubernetes API
  endpoint mechanism is an unrelated, much larger change than a config-representation migration,
  and this cluster already has BGP peering used for other purposes; swapping the control-plane VIP
  onto it is a deliberate future decision, not a side effect of dropping talhelper.
- **`KubeAdmissionControlConfig` omitted entirely** — its own ADR,
  [ADR-0012](0012-omit-kubeadmissioncontrolconfig-in-talos-multidoc-migration.md), since it's a
  security-posture call worth a dedicated paper trail rather than a line in this one.

## Considered options

- **Migrate to a talhelper-adjacent successor tool** (`topf`, `talstomize` — both suggested by
  talhelper's own archival notice). Rejected: both are new, unfamiliar dependencies for a 3-node
  cluster with no staging environment; `talosctl machineconfig patch` is Talos's own, officially
  supported, already-installed mechanism for exactly this job, so adding a third-party layer on
  top buys nothing here.
- **Keep talhelper, work around the v1.14 collision with more delete-patches.** Rejected: the
  collision isn't a single fixable patch — it's five of them, one per legacy field now
  auto-populated by the default base — and doesn't address talhelper being unmaintained with no
  path to the 4 multi-doc kinds it's missing outright (in particular `KubeNodeConfig`, which this
  migration needs for node labels/taints).
- **Externalize secrets to 1Password like onedr0p**, for closer parity with the reference
  implementation. Rejected: no existing 1Password Connect/`op` CLI wiring anywhere in this repo;
  adding one is a real new dependency this migration doesn't need — SOPS-in-repo is this repo's
  existing, working secret-storage pattern for everything else.

## Consequences

- `talos/talconfig.yaml`, `talos/talenv.yaml`, `talos/patches/`,
  `.taskfiles/talos/Taskfile.yaml`, and the `talos: .taskfiles/talos` include in the root
  `Taskfile.yaml` are deleted. `talos/mod.just` + a root `.justfile` (`mod talos`) take over;
  `just talos <recipe>` replaces `task talos:<task>`.
- Regenerating any node's config is no longer a single opaque `talhelper genconfig` call — it's
  `talosctl gen config` (base) piped through `talosctl machineconfig patch` (layered patches),
  both plain `talosctl` subcommands with no external tool in between.
- **Legacy `bootstrap/` tooling is untouched and still depends on talhelper**:
  `.taskfiles/bootstrap/Taskfile.yaml`'s `talos` task (`talhelper gensecret`/`genconfig`/
  `gencommand ...`) operates on a separate, template-scaffolded
  `kubernetes/bootstrap/talos/` tree used only for bringing up a cluster from scratch — not the
  live `talos/` tree this ADR covers. Per this repo's existing convention (`bootstrap/` is
  legacy/one-time tooling), it's out of scope here. If that from-scratch path is ever exercised
  again, it will hit the same v1.14 collision this ADR describes and need the same fix — flagged
  here so it isn't a surprise, not fixed now.
  `.taskfiles/workstation/Taskfile.yaml`/`Brewfile`/`Archfile` still install the `talhelper`
  binary for that same reason and are likewise untouched.
- Renovate's version pins move: `talosVersion`/`kubernetesVersion` (previously in
  `talconfig.yaml`, `# renovate: datasource=... depName=...` comments) move to a new
  `talos/versions.yaml`, keeping the exact same comment convention so the existing
  `home-operations/renovate-presets` custom managers (the generic annotated-dependency regex,
  which matches any file, and the `talosFactory` preset's `talosVersion:` regex, scoped to
  `talos/*.yaml`) keep bumping them without any Renovate config change.
