# Talos v1.14 GA Multi-Document Config Kind Schema — Primary Source Verification

Research for issue [#1506](https://github.com/rwade628/homelab/issues/1506), a child of the
wayfinder map issue [#1504](https://github.com/rwade628/homelab/issues/1504) ("Map: Migrate Talos
machine config off talhelper to multi-doc kinds"). Feeds the target-structure decision in
[#1508](https://github.com/rwade628/homelab/issues/1508). Cross-check against the
`onedr0p/home-ops` worked example is out of scope here (see sibling ticket #1505); this file is
the canonical/official-source side only.

`docs/research/` already exists in this repo (`rook-v120-k8s-compat.md`,
`rook-v120-storageclass-params.md`) — this file follows that convention.

## Method / sources

- **`siderolabs/talos` source, cloned at the `v1.14.0` tag** (`git clone --depth 1 --branch
  v1.14.0 https://github.com/siderolabs/talos.git`, resolved commit `9abd05a`). All `pkg/machinery/...`
  and `website/content/v1.14/...` paths below are file paths inside that checkout unless a full
  URL is given.
- **Talos's own generated v1.14 docs**, read straight from `website/content/v1.14/reference/configuration/**/*.md`
  in that same checkout — this is the source Hugo builds the public docs from, byte-for-byte.
  Cross-checked live at `https://docs.siderolabs.com/talos/v1.14/reference/configuration/...`
  (the docs site moved off `talos.dev`, which 404s for `/v1.14/...` as of 2026-09-05 — the
  `docs.siderolabs.com` mirror returned identical field tables).
- `CHANGELOG.md` at the `v1.14.0` tag (the actual GA release notes, not a summary of them).
- `pkg/machinery/config/contract.go` — the version-gate logic that `talosctl gen config` (and
  anything using the machinery's generator, including talhelper) consults to decide legacy vs.
  multi-doc output.
- `budimanjojo/talhelper`'s own `go.mod` and GitHub release, via `gh api`.
- This repo's current state: `talos/talconfig.yaml` (`talosVersion: v1.14.0`),
  `talos/patches/controller/cluster.yaml`, `talos/patches/global/machine-kubelet.yaml`,
  `talos/patches/global/machine-labels.yaml` (all present, unchanged, as of this research).

## Headline answer

**Talos v1.14.0 GA is squarely the intended cutover point for these document kinds.**
`pkg/machinery/config/contract.go` gates them behind a single version check:

```go
// MultidocKubernetesConfigSupported returns true if version of Talos should use multi-doc Kubernetes config.
func (contract *VersionContract) MultidocKubernetesConfigSupported() bool {
	return contract.Greater(TalosVersion1_13)
}
```
(`pkg/machinery/config/contract.go:280-283`, also present verbatim at the `v1.14.0-alpha.2` tag —
this gate itself isn't new in the GA release, see the talhelper section below for why that matters).
`TalosVersion1_13 = &VersionContract{1, 13}` (`contract.go:29`), so this is `true` for any
`talosVersion` `> 1.13`, i.e. `1.14.0` and later — exactly this cluster's pinned version.

The official GA changelog confirms this is a real cutover, not just an internal flag, and gives an
explicit deprecation table (`CHANGELOG.md:282-316`, "### Kubernetes Multi-document Configuration"):

> Talos introduces new multi-document Kubernetes configuration, which allows for more flexible and
> modular configuration of Kubernetes components. Talos still supports the old v1alpha1 config for
> backwards compatibility, but new features and fields will only be available in the new
> multi-document format.

**Caution on "is it GA":** every one of these documents still declares `apiVersion: v1alpha1` in
its own YAML (e.g. the `KubeAPIServerConfig` example in
`website/content/v1.14/reference/configuration/kubernetes/kubeapiserverconfig.md:17-24`). That
`v1alpha1` is Talos's document-schema-versioning label, shared by every config document kind Talos
has ever shipped (stable ones included, e.g. `NetworkConfig`, `TimeConfig`) — it is **not** a
"this feature is alpha" marker. The actual stability signal is the `contract.go` gate above plus
the absence of any "alpha"/"experimental" callout in the doc source or the CHANGELOG entry (there
is none for any of the kinds below). Don't let the literal string `v1alpha1` in example YAML read
as a maturity warning.

## Per-document-kind findings

### `KubeAPIServerConfig`

- **GA in 1.14.0**: yes. File existed already at `v1.14.0-alpha.2` (`git cat-file -e
  v1.14.0-alpha.2:pkg/machinery/config/types/k8s/apiserver.go` succeeds), so it predates even the
  alpha talhelper is pinned to — no risk there specifically.
- **Fields** (`pkg/machinery/config/types/k8s/apiserver.go:53-99`, doc:
  `website/content/v1.14/reference/configuration/kubernetes/kubeapiserverconfig.md`):
  `image`, `extraArgs` (`Args`, i.e. our `feature-gates`/`runtime-config` map), `env`,
  `resources` (`requests`/`limits`), `apiPort` (default 6443), `certExtraSANs`, `startupProbes`.
  **No `enable-aggregator-routing`-specific field** — that stays a generic `extraArgs` entry, same
  as today.
- **Admission control**: `KubeAPIServerConfig` itself has **no** `admissionControl` field at all
  (confirmed absent from the struct). It moved to a **separate, one-document-per-plugin** kind:
  **`KubeAdmissionControlConfig`** (`pkg/machinery/config/types/k8s/admission_control.go:47-59`,
  doc: `.../kubernetes/kubeadmissioncontrolconfig.md`) — fields `name` (plugin name, e.g.
  `PodSecurity`) and `configuration` (`Unstructured`, the literal admission-plugin config object).
  `talosctl gen config` emits one such document by default for `PodSecurity`
  (`DefaultPodSecurityAdmissionControlConfig()`, `admission_control.go:69-90`). This is the
  multi-doc replacement for the legacy `cluster.apiServer.admissionControl` list that this repo's
  removed `$$patch: delete` was targeting — CHANGELOG confirms: "Deprecated `.cluster.apiServer`
  in the v1alpha1 config; use the `KubeAPIServerConfig`, `KubeAdmissionControlConfig`,
  `KubeAuditPolicyConfig`, `KubeAuthenticationConfig` and `KubeAuthorizerConfig` documents"
  (`CHANGELOG.md:291`).

**Mapping for our fields**: `extraArgs.enable-aggregator-routing`, `.feature-gates`,
`.runtime-config` → `KubeAPIServerConfig.extraArgs` verbatim, unchanged keys.

### `KubeControllerManagerConfig`

- **GA in 1.14.0**: yes, and already present at `v1.14.0-alpha.2`.
- **Fields** (`pkg/machinery/config/types/k8s/controller_manager.go`, doc:
  `.../kubeschedulerconfig.md`... actually `.../kubecontrollermanagerconfig.md`): `enabled` (bool,
  default true — set false to disable the static pod if it runs elsewhere), `image`, `extraArgs`,
  `env`, `resources`.
- **Mapping**: `extraArgs.bind-address: 0.0.0.0` → `KubeControllerManagerConfig.extraArgs`
  verbatim.

### `KubeSchedulerConfig`

- **GA in 1.14.0**: yes, already present at `v1.14.0-alpha.2`.
- **Fields** (`pkg/machinery/config/types/k8s/scheduler.go`, doc: `.../kubeschedulerconfig.md`):
  `enabled` (bool, default true), `image`, `config` (`Unstructured` — full
  `KubeSchedulerConfiguration`, preferred over `extraArgs` per the doc's own note), `extraArgs`,
  `env`, `resources`.
- **Mapping**: `extraArgs.bind-address: 0.0.0.0` → `KubeSchedulerConfig.extraArgs` verbatim.

### `KubeProxyConfig`

- **GA in 1.14.0**: yes, already present at `v1.14.0-alpha.2`.
- **Fields** (doc: `.../kubeproxyconfig.md`): `enabled` (bool, **default true** — this is the
  `disabled`/`enabled` equivalent: our `proxy.disabled: true` maps to `enabled: false`), `image`,
  `mode` (`iptables`/`ipvs`/`nftables`, default `nftables`), `config` (Unstructured kube-proxy
  config), `extraArgs`, `resources`. CHANGELOG notes kube-proxy now uses `config` for most
  settings, not flags: "The `kube-proxy` is now using configuration to manage its settings instead
  of command line arguments (with new `KubeProxyConfig` document)" (`CHANGELOG.md:286`).
- **Mapping**: `cluster.proxy.disabled: true` → `KubeProxyConfig.enabled: false`.

### `KubeCoreDNSConfig`

- **GA in 1.14.0**: yes, but **added after `v1.14.0-alpha.2`** (`git cat-file -e
  v1.14.0-alpha.2:pkg/machinery/config/types/k8s/coredns.go` fails — path doesn't exist at that
  tag; present in the final `v1.14.0` tree).
- **Fields** (doc: `.../kubecorednsconfig.md`): `enabled` (bool, **default true**), `image`.
- **Mapping**: `cluster.coreDNS.disabled: true` → `KubeCoreDNSConfig.enabled: false`.

### `KubeletConfig`

- **GA in 1.14.0**: yes, but **also added after `v1.14.0-alpha.2`** (same non-existence check,
  `pkg/machinery/config/types/k8s/kubelet.go` missing at that tag).
- **Fields** (`pkg/machinery/config/types/k8s/kubelet.go:57-96`, doc: `.../kubeletconfig.md`):
  `image`, `config` (`Unstructured`, the raw upstream kubelet config API object — this is where
  `serializeImagePulls`/`imageMaximumGCAge` go, keyed exactly as the kubelet's own config schema),
  `extraArgs` (this is where `allowed-unsafe-sysctls` goes, since it's a kubelet flag not a config
  field), `clusterDNS`, `defaultRuntimeSeccompProfileEnabled`.
- **Important correction to the ticket's assumption: `KubeletConfig` has no `nodeIP` field at
  all** — it was deliberately *not* kept here. `nodeIP.validSubnets` moved to **`KubeNodeConfig`**
  instead (see below), confirmed by (a) the struct in `kubelet.go` having no such field and (b)
  CHANGELOG's explicit deprecation list: "`.machine.kubelet.nodeIP`" is listed under "Deprecated
  the following list of fields, all of them moved into `KubeNodeConfig`" (`CHANGELOG.md:299-306`).
- **Mapping**:
  - `machine.kubelet.extraArgs.allowed-unsafe-sysctls` → `KubeletConfig.extraArgs`, unchanged.
  - `machine.kubelet.extraConfig.{serializeImagePulls,imageMaximumGCAge}` → `KubeletConfig.config`
    (same keys — `config` is literally the kubelet config object, `extraConfig` was Talos's old
    name for the same pass-through).
  - `machine.kubelet.nodeIP.validSubnets` → **`KubeNodeConfig.nodeIP.validSubnets`**, *not*
    `KubeletConfig`.

### `KubeNodeConfig`

- **GA in 1.14.0**: yes, brand new in this release (did not exist at `v1.14.0-alpha.2`).
- **Fields** (`pkg/machinery/config/types/k8s/node.go:54-95`, doc: `.../kubenodeconfig.md`):
  `skipNodeRegistration`, `registerWithFQDN`, `nodeIP` (`{validSubnets: []string}`), `labels`
  (`map[string]string`), `annotations`, `taints` (`map[string]string`, value is
  `<value>:<effect>`, effect optional).
- **Confirms node labels moved here, not stayed under legacy `machine.nodeLabels`.** The
  `V1Alpha1ConflictValidate` method on this type (`node.go:187-225`) explicitly errors if
  `.machine.nodeLabels` / `.machine.nodeAnnotations` / `.machine.nodeTaints` /
  `.machine.kubelet.{skipNodeRegistration,registerWithFQDN,nodeIP}` are *also* set — i.e. once you
  adopt `KubeNodeConfig` you must fully migrate off all of those legacy fields simultaneously, not
  partially. Same CHANGELOG list as above (`CHANGELOG.md:299-306`) states this in plain English.
- **`allowSchedulingOnControlPlanes` — resolves to `KubeNodeConfig`, but as taint data, not a
  boolean field.** `KubeNodeConfigV1Alpha1` has no `allowSchedulingOnControlPlanes`-named field.
  Instead, per CHANGELOG.md:300 ("`.cluster.allowSchedulingOnControlPlanes`" is in the same
  "moved into `KubeNodeConfig`" list) and line 307 ("The default `NoSchedule` taint for
  controlplane and label are now explicitly listed in `KubeNodeConfig`"), the mechanism —
  confirmed directly in the generator source
  (`pkg/machinery/config/generate/kubernetes.go:166-182`) — is:
  ```go
  nodeConfig := k8s.NewKubeNodeConfigV1Alpha1()
  if isControlplane {
      nodeConfig.LabelsConfig = map[string]string{
          constants.LabelNodeRoleControlPlane: "", // "node-role.kubernetes.io/control-plane"
      }
  }
  if isControlplane && !in.Options.AllowSchedulingOnControlPlanes {
      nodeConfig.TaintsConfig = map[string]string{
          constants.LabelNodeRoleControlPlane: constants.TaintEffectNoSchedule, // ":NoSchedule"
      }
  }
  ```
  i.e. control-plane nodes always get the `node-role.kubernetes.io/control-plane: ""` label in
  `KubeNodeConfig.labels`; the `node-role.kubernetes.io/control-plane:NoSchedule` taint is added to
  `KubeNodeConfig.taints` **only when scheduling should be disallowed**. Since this cluster sets
  `allowSchedulingOnControlPlanes: true`, the multi-doc equivalent is a control-plane `KubeNodeConfig`
  with the control-plane label set and **no** taint entry (i.e. `taints` omitted/empty) — there is
  no dedicated toggle field to carry forward, only the presence/absence of that one taint.
- **Mapping**:
  - `machine.nodeLabels.topology.kubernetes.io/region: main` (global) and per-node
    `feature.node.kubernetes.io/amd-gpu`/`topology.kubernetes.io/zone` → `KubeNodeConfig.labels`
    on the relevant node(s) (this doc is per-node like every Talos machine config document, so
    per-node values are just per-node documents, no separate templating construct needed).
  - `cluster.allowSchedulingOnControlPlanes: true` → control-plane `KubeNodeConfig` with the
    `node-role.kubernetes.io/control-plane` label present and the matching taint entry **absent**.

### `SecurityProfileConfig`

- **GA in 1.14.0**: yes, but **brand new in the GA release** — did not exist at
  `v1.14.0-alpha.2` (`pkg/machinery/config/types/runtime/security_profile_config.go` absent at
  that tag; this is the single biggest gap for talhelper, see below). It lives under
  `types/runtime`, not `types/security` (there is a separate, unrelated `security/` doc dir for
  `ImageVerificationConfig`/`TrustedRootsConfig`).
- **Field**: `workloadIsolation` (bool) only
  (`pkg/machinery/config/types/runtime/security_profile_config.go:59-67`, doc:
  `website/content/v1.14/reference/configuration/runtime/securityprofileconfig.md`).
- **Default value when the document is absent: `false`** (non-isolated / old behavior).
  `WorkloadIsolation()` does `pointer.SafeDeref(s.WorkloadIsolationEnabled)` which returns the
  zero value (`false`) for a `nil` pointer (`security_profile_config.go:100-102`) — but that's only
  reachable if the document exists with the field unset; if the *document itself* is absent, the
  node simply runs the pre-1.14 non-sandboxed code path (confirmed by
  `contract.go:293-296`: `WorkloadIsolationEnabledByDefault()` — gates whether **`talosctl gen
  config`** synthesizes the document with `workloadIsolation: true` for *newly generated* configs;
  it does not retroactively add the document to an existing/upgraded machine config). Doc text is
  explicit: "`talosctl gen config` emits this document with `workloadIsolation: true` for Talos
  1.14+, so new clusters are isolated by default; clusters upgraded from older versions do not have
  the document and keep the old (non-isolated) behavior unless it is added" (doc file lines 11-13,
  verbatim also in `CHANGELOG.md:435-439`).
- **This repo currently has no such document** (`talos/patches/` has no
  `machine-security-profile.yaml` and `talconfig.yaml` has no `security` mention as of this
  research) — matching the ticket's premise: workload isolation is implicitly **off** right now
  (equivalent to the pre-1.14 default), not explicitly pinned either way. To keep current behavior
  explicitly (recommended, since it's otherwise implicit/fragile against a future talhelper
  regenerate or manual `talosctl gen config` re-run) the document to add is:
  ```yaml
  apiVersion: v1alpha1
  kind: SecurityProfileConfig
  workloadIsolation: false
  ```

### `cluster.etcd` (extraArgs / advertisedSubnets)

- **No multi-doc kind exists for this in v1.14.0.** Searched the entire
  `pkg/machinery/config/types/` tree for anything etcd-related: the only hit is
  `KubeEtcdEncryptionConfig` (`pkg/machinery/config/types/k8s/etcd_encryption.go`), which only
  covers etcd **encryption-at-rest** configuration (replacing
  `.cluster.secretboxEncryptionSecret` per `CHANGELOG.md:290`) — nothing about `extraArgs`,
  `advertisedSubnets`, or any other etcd runtime setting. The full etcd config type
  (`EtcdConfig`, with `ExtraArgs`/`AdvertisedSubnets`) only exists in the legacy monolithic
  `pkg/machinery/config/types/v1alpha1/v1alpha1_etcdconfig.go`, and the "Kubernetes
  Multi-document Configuration" changelog section (`CHANGELOG.md:282-316`) does not mention
  `.cluster.etcd` anywhere in its deprecation list.
- **Conclusion**: `cluster.etcd.extraArgs` (`listen-metrics-urls`) and
  `cluster.etcd.advertisedSubnets` **stay under the legacy `cluster:` block** — there is nothing
  to migrate here yet in v1.14.0 GA.

## talhelper: does it actually understand these kinds for v1.14.0 GA?

**No — talhelper v3.1.17 predates and is missing several of these kinds entirely, and it will
never be updated.**

- `gh api repos/budimanjojo/talhelper/releases/tags/v3.1.17` — the release body states: *"This
  will be the last release of Talhelper... This repo will now be archived, and I suggest people
  who depend on this tool to migrate to other similar tool like
  [topf](https://github.com/postfinance/topf) and
  [talstomize](https://github.com/mirceanton/talstomize)."* talhelper is unmaintained as of this
  release — there will be no future release to catch up to GA.
- `gh api repos/budimanjojo/talhelper/contents/go.mod?ref=v3.1.17` pins
  `github.com/siderolabs/talos/pkg/machinery v1.14.0-alpha.2` — confirmed, matching issue #1501's
  premise exactly.
- Diffing that exact alpha tag against the `v1.14.0` GA tag in the real `siderolabs/talos` repo
  (`git cat-file -e v1.14.0-alpha.2:<path>` for each relevant type file) shows talhelper's
  embedded machinery is **missing five of the nine kinds this ticket cares about outright** — the
  Go types didn't exist yet when that alpha was cut, so talhelper cannot know their field names,
  validate them, or serialize them, even if a user hand-writes a raw patch document of that kind:

  | Kind | Exists at `v1.14.0-alpha.2` (talhelper's pin) |
  |---|---|
  | `KubeAPIServerConfig` | yes |
  | `KubeAdmissionControlConfig` | yes |
  | `KubeControllerManagerConfig` | yes |
  | `KubeSchedulerConfig` | yes |
  | `KubeProxyConfig` | yes |
  | `KubeCoreDNSConfig` | **no** (added post-alpha.2) |
  | `KubeletConfig` | **no** (added post-alpha.2) |
  | `KubeNodeConfig` | **no** (added post-alpha.2) |
  | `KubeClusterConfig` | **no** (added post-alpha.2) |
  | `SecurityProfileConfig` | **no** (added post-alpha.2) |

  Notably, the two kinds this ticket's decision hinges on most — `KubeNodeConfig` (node
  labels/taints/nodeIP, and the `allowSchedulingOnControlPlanes` replacement) and
  `SecurityProfileConfig` (`workloadIsolation`) — are both in the "missing" set. talhelper as
  currently pinned cannot be relied on to correctly round-trip either one, on top of already being
  end-of-life upstream.
- This corroborates and sharpens issue #1501's premise: it's not just that talhelper is "a bit
  behind" GA, it's specifically blind to the node-config and security-profile multi-doc kinds
  that matter most for this migration, and there is no future talhelper release coming to fix
  that.

## Summary table (current legacy field → v1.14 GA multi-doc target)

| Current legacy field | Multi-doc kind | Field | Notes |
|---|---|---|---|
| `cluster.allowSchedulingOnControlPlanes` | `KubeNodeConfig` | `taints` (absence of the control-plane `NoSchedule` entry) + `labels` | No boolean field; presence/absence of one taint key |
| `cluster.apiServer.extraArgs.*` | `KubeAPIServerConfig` | `extraArgs` | Direct, same keys |
| `cluster.apiServer.admissionControl` (removed) | `KubeAdmissionControlConfig` | one document per plugin (`name`, `configuration`) | Structurally different: list → N documents |
| `cluster.controllerManager.extraArgs.*` | `KubeControllerManagerConfig` | `extraArgs` | Direct |
| `cluster.coreDNS.disabled` | `KubeCoreDNSConfig` | `enabled` (inverted, default true) | |
| `cluster.etcd.*` | — | — | **No multi-doc kind in v1.14.0**; stays legacy |
| `cluster.proxy.disabled` | `KubeProxyConfig` | `enabled` (inverted, default true) | |
| `cluster.scheduler.extraArgs.*` | `KubeSchedulerConfig` | `extraArgs` | Direct |
| `machine.kubelet.extraArgs.*` | `KubeletConfig` | `extraArgs` | Direct |
| `machine.kubelet.extraConfig.*` | `KubeletConfig` | `config` | Direct (raw kubelet config object) |
| `machine.kubelet.nodeIP.validSubnets` | `KubeNodeConfig` (not `KubeletConfig`) | `nodeIP.validSubnets` | Moved to a different document than expected |
| `machine.nodeLabels.*` (global + per-node) | `KubeNodeConfig` | `labels` | Per-node, since the doc itself is per-node |

## Talhelper repo location note

Source archive was cloned to a scratch path outside the repo
(`/tmp/claude-1000/.../scratchpad/talos-src/talos`, not committed) purely for `git show`/`git
cat-file` inspection; no files from it were copied into this repository.
