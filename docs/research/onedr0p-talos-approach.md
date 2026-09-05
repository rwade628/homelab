# Research: `onedr0p/home-ops` Talos multi-doc approach

**Ticket**: #1505 (child of map #1504 — "Migrate Talos machine config off talhelper to multi-doc
kinds"). Feeds decision tickets #1508 (document-kind structure) and #1509 (talhelper replacement
mechanics, secret carry-over, per-node templating).

**Source**: `github.com/onedr0p/home-ops`, cloned shallow at commit
`1dc1cd521524be564eacf989c25489024e182fa7` (2026-09-04, default branch `main`). All line numbers
below refer to that commit.

---

## 1. File structure

Everything lives under `talos/` (9 files total), driven by a `just` module, not a standalone
script. There is no `talconfig.yaml`-equivalent single source file — the fleet's shared state is
composed from a small, explicit stack of Jinja templates:

| File | Purpose |
|---|---|
| `talos/cluster.yaml.j2` | Documents applied to **every** node (base secrets refs, network link/bond/VLAN config, disk/install config, sysctls, kernel modules, shared `KubeNodeConfig`/`KubeletConfig`/`KubeNetworkConfig`/`KubePrismConfig`) — `talos/cluster.yaml.j2:1-204` |
| `talos/controlplane.yaml.j2` | Control-plane-only documents, **including `machine.type: controlplane`** and all the `Kube*Config` API-server/controller-manager/scheduler/coreDNS/proxy/etcd-encryption/API-access documents — `talos/controlplane.yaml.j2:1-127` |
| `talos/nodes/controlplane/{k8s-0,k8s-1,k8s-2}.yaml.j2` | Per-node documents: just `HostnameConfig` + a `KubeNodeConfig` with a zone label — `talos/nodes/controlplane/k8s-0.yaml.j2:1-10` |
| `talos/schematic.yaml.j2` | Shared Image Factory schematic (kernel args, system extensions) — `talos/schematic.yaml.j2:1-17` |
| `talos/mod.just` | `just talos ...` recipes (render/apply/upgrade/reset/schematic lookups) |
| `talos/README.md` | Documents the layering model and gotchas |
| `talos/nodes/workers/` | **Does not exist yet** — README (`talos/README.md:13`) says it's created only "with the first worker"; there's a `workers.yaml.j2` role-patch referenced but likewise absent today. This is a control-plane-only (3-node) cluster currently. |

Composition happens via `talosctl machineconfig patch`, wired up in `talos/mod.just:26-34`
(`render-config` recipe):

```sh
talosctl machineconfig patch <(just template "{{ source_directory() }}/cluster.yaml.j2" -D "schematic=${schematic}") \
    -p @<(just template "{{ source_directory() }}/${role}.yaml.j2") \
    -p @<(just template "{{ source_directory() }}/nodes/${role}/{{ node }}.yaml.j2")
```

`role` is derived from **directory placement** (`talos/mod.just:27-31`): if
`nodes/workers/<node>.yaml.j2` exists, role=`workers`, else `controlplane`. README calls this out
explicitly as a convention: "Directory placement is the single source of truth for a node's role...
A node cannot claim one role by filename and another by content" (`talos/README.md:35-37`).

Each layer is piped through a private root-level `just` recipe before `talosctl` sees it
(`.justfile:23-24` in the repo root, not under `talos/`):

```
template file *args:
    minijinja-cli "{{ file }}" {{ args }} | op inject
```

So the real pipeline per layer is: **Jinja render (`minijinja-cli`, strict mode) → 1Password
secret substitution (`op inject`) → fed to `talosctl machineconfig patch` as a file/process-sub
argument**. `talosctl` itself does the multi-doc strategic merge: "documents with the same kind/name
are deep-merged, new documents are appended" (`talos/README.md:33`).

Bootstrap (`bootstrap/README.md:216-256`) calls this per-node render as stage 1 ("nodes") of
`just bootstrap cluster`, applying with `talosctl apply-config --insecure` for brand-new nodes.

## 2. Document-kind mapping

| Legacy field (our `talhelper` config) | onedr0p multi-doc equivalent | Citation |
|---|---|---|
| `cluster.apiServer.extraArgs` | `kind: KubeAPIServerConfig`, `extraArgs:` (+ `certExtraSANs:`, `image:`) | `talos/controlplane.yaml.j2:32-39` |
| `cluster.apiServer.admissionControl` | **Absent.** No `KubeAdmissionControlConfig` (or `SecurityProfileConfig`) document exists anywhere under `talos/`. onedr0p doesn't customize admission control at all — they rely on Talos's built-in default chain rather than an explicit `admissionControl:`/removal patch like our `talos/patches/controller/admission-controller-patch.yaml`. | grep across `talos/` for `AdmissionControl`/`SecurityProfile` returns nothing |
| `cluster.controllerManager.extraArgs` | `kind: KubeControllerManagerConfig`, `extraArgs:` (+ `image:`) | `talos/controlplane.yaml.j2:70-75` |
| `cluster.scheduler.extraArgs` | `kind: KubeSchedulerConfig`, `extraArgs:` (+ a `config:` block carrying scheduler-plugin config, `image:`) | `talos/controlplane.yaml.j2:99-118` |
| `cluster.coreDNS.disabled` | `kind: KubeCoreDNSConfig`, **`enabled: false`** — note the polarity flip: legacy is `disabled: true`, new doc is `enabled: false` | `talos/controlplane.yaml.j2:77-79` |
| `cluster.proxy.disabled` | `kind: KubeProxyConfig`, **`enabled: false`** (+ `image:`) — same polarity flip as coreDNS | `talos/controlplane.yaml.j2:94-97` |
| `machine.kubelet.extraArgs` | Would be `kind: KubeletConfig`, `extraArgs:` — but **onedr0p doesn't use any kubelet extraArgs today**, so there's no real example to quote. The document that exists only sets `config:` (a KubeletConfiguration-shaped block: `crashLoopBackOff`, `imageMaximumGCAge`, `maxParallelImagePulls`, `serializeImagePulls`, `shutdownGracePeriod`, `shutdownGracePeriodCriticalPods`), `defaultRuntimeSeccompProfileEnabled`, `image`. | `talos/cluster.yaml.j2:180-191` |
| `machine.kubelet.extraConfig` | Same `KubeletConfig.config:` block noted above appears to be the new home for what was `extraConfig` (arbitrary kubelet config overrides) — quoted verbatim above | `talos/cluster.yaml.j2:181-190` |
| `machine.kubelet.nodeIP.validSubnets` | **Moved out of `KubeletConfig` entirely** — now lives on `kind: KubeNodeConfig`'s `nodeIP.validSubnets:` | `talos/cluster.yaml.j2:171-175` |
| `machine.nodeLabels` | `kind: KubeNodeConfig`, `labels:` — and it's **deliberately split across three layers that deep-merge**: cluster-wide labels in `cluster.yaml.j2` (`node.kubernetes.io/gpu`, `topology.kubernetes.io/region`), a control-plane-role label in `controlplane.yaml.j2` (`node-role.kubernetes.io/control-plane: ""`), and a per-node zone label in each `nodes/controlplane/<node>.yaml.j2` (`topology.kubernetes.io/zone: m`) | `talos/cluster.yaml.j2:171-178`, `talos/controlplane.yaml.j2:27-30`, `talos/nodes/controlplane/k8s-0.yaml.j2:6-9` |

**`SecurityProfileConfig` / `KubeAdmissionControlConfig`**: confirmed absent (see admission-control
row above). This is itself the informative datapoint the ticket asked about — onedr0p's reference
implementation gives no real-world example of expressing our
`admission-controller-patch.yaml`/`cluster.yaml`'s `admissionControl: $$patch: delete` in the new
kind model; #1508 will need to resolve that gap independently (either find the actual Talos
multi-doc kind for admission plugin chain config from Talos's own docs/source, or decide the
default chain is acceptable and drop the customization like onedr0p did).

## 3. Secret handling

**onedr0p does not commit or SOPS-encrypt a `talosctl gen secrets`-style bundle in-repo at all.**
Every sensitive value — `MACHINE_CA_CRT`/`KEY`, `MACHINE_TOKEN`, `CLUSTER_CA_CRT`/`KEY`,
`CLUSTER_TOKEN`, `CLUSTER_AGGREGATORCA_CRT`/`KEY`, `CLUSTER_ETCD_CA_CRT`/`KEY`,
`CLUSTER_SERVICEACCOUNT_KEY`, `CLUSTER_SECRETBOXENCRYPTIONSECRET`, `CLUSTER_ID`, `CLUSTER_SECRET` —
is referenced as an `op://kubernetes/talos/<FIELD>` URI (1Password vault `kubernetes`, item
`talos`) directly inline in the templates:

```yaml
machine:
  ca:
    crt: op://kubernetes/talos/MACHINE_CA_CRT
  token: op://kubernetes/talos/MACHINE_TOKEN
cluster:
  ca:
    crt: op://kubernetes/talos/CLUSTER_CA_CRT
  token: op://kubernetes/talos/CLUSTER_TOKEN
```
— `talos/cluster.yaml.j2:2-17`

```yaml
machine:
  ca:
    crt: op://kubernetes/talos/MACHINE_CA_CRT
    key: op://kubernetes/talos/MACHINE_CA_KEY
cluster:
  ca:
    crt: op://kubernetes/talos/CLUSTER_CA_CRT
    key: op://kubernetes/talos/CLUSTER_CA_KEY
  aggregatorCA:
    crt: op://kubernetes/talos/CLUSTER_AGGREGATORCA_CRT
    key: op://kubernetes/talos/CLUSTER_AGGREGATORCA_KEY
  etcd:
    ca:
      crt: op://kubernetes/talos/CLUSTER_ETCD_CA_CRT
      key: op://kubernetes/talos/CLUSTER_ETCD_CA_KEY
  serviceAccount:
    key: op://kubernetes/talos/CLUSTER_SERVICEACCOUNT_KEY
```
— `talos/controlplane.yaml.j2:2-25`

The `README.md` states this as a deliberate convention: "**Secrets never live in this repo.** All
sensitive values are `op://kubernetes/talos/...` references resolved at render time"
(`talos/README.md:38-39`), and `bootstrap/README.md:18-20` repeats it: "Machine secrets never live
in this repo; every `op://` reference in the Talos configs and bootstrap manifests is resolved at
apply time with `op inject`."

Resolution mechanism: the root `.justfile`'s `template` recipe pipes the Jinja-rendered YAML through
`op inject` (`.justfile:23-24`), which is 1Password CLI's built-in "replace `op://vault/item/field`
tokens with live secret values" feature. There is **no `talosctl gen secrets` invocation anywhere in
the repo** (`talos/`, `bootstrap/`, or root) — the generation step, whatever it was, happened once,
out-of-band, and the resulting fields were manually loaded into the 1Password item. Nothing in the
repo documents that one-time step; it's simply assumed to already exist as a precondition (an
already-populated `kubernetes/talos` 1Password item), the same way `bootstrap/README.md:18-27`
assumes a pre-existing `talosconfig` and a signed-in `op` CLI.

The field names used (`MACHINE_CA_CRT/KEY`, `CLUSTER_CA_CRT/KEY`, `CLUSTER_TOKEN`,
`CLUSTER_AGGREGATORCA_*`, `CLUSTER_ETCD_CA_*`, `CLUSTER_SERVICEACCOUNT_KEY`,
`CLUSTER_SECRETBOXENCRYPTIONSECRET`, `CLUSTER_ID`, `CLUSTER_SECRET`) map 1:1 onto the fields
`talosctl gen secrets` (and by extension talhelper's `talsecret.sops.yaml`, which wraps the same
generator) produces. That is the answer to the "existing cluster, no regeneration" question: **the
underlying secret material is identical between the two tools** — talhelper doesn't invent its own
crypto, it shells out to the same Talos secrets-bundle format. So for our LIVE cluster, the
multi-doc migration does not require running `talosctl gen secrets` again; it requires extracting
the already-generated values (today sitting in `talos/talsecret.sops.yaml`, produced by talhelper)
and re-referencing them from the new template layer — either by keeping them in a SOPS-encrypted
YAML checked into this repo (our existing pattern) and templating with a Jinja `vars`/lookup
mechanism, or by lifting them out to an external secret store the way onedr0p does with 1Password.
Ticket #1509 should treat "commit encrypted vs. externalize to a secret manager" as the open
decision, not "regenerate vs. don't" — regeneration is never in play either way, only where the
existing values live.

One structural gotcha worth carrying into #1509: `talos/README.md:53-54` — "`machine.ca` and
`cluster.ca` merge as a cert+key **unit**: a patch supplying only `key` blanks `crt`. This is why
`controlplane.yaml.j2` repeats the `crt` references alongside the keys." Any layered-patch design we
adopt needs the same repetition or it will silently null out the CA certificate on merge.

## 4. Per-node templating

This is the most surprising finding relative to the ticket's framing: **onedr0p does not use a
central per-node vars file (no `talenv.yaml`/`talconfig.yaml`-nodes-array equivalent) at all.**
Instead, per-node differences are expressed as **one physical `.j2` file per node**
(`talos/nodes/controlplane/k8s-0.yaml.j2`, `k8s-1.yaml.j2`, `k8s-2.yaml.j2`), each containing
literal, non-parameterized YAML:

```yaml
---
apiVersion: v1alpha1
kind: HostnameConfig
hostname: k8s-0
---
apiVersion: v1alpha1
kind: KubeNodeConfig
labels:
  topology.kubernetes.io/zone: m
```
— `talos/nodes/controlplane/k8s-0.yaml.j2:1-9` (k8s-1/k8s-2 are identical except `hostname:`)

Grepping every `.j2` file under `talos/` for Jinja syntax (`{{`/`{%`) turns up exactly **one** use
in the whole tree:

```yaml
image: factory.talos.dev/metal-installer/{{ schematic }}:v1.14.0
```
— `talos/cluster.yaml.j2:74`, where `schematic` is passed as a `-D "schematic=..."` define
(`talos/mod.just:32`, computed by POSTing the rendered `schematic.yaml.j2` to the Image Factory API).

Consequences for the specific per-node values #1509 asks about:

- **Bond interface hardware MAC selector**: not per-node at all in onedr0p's setup. It's a single
  shared `LinkAliasConfig` in `cluster.yaml.j2` matching a vendor MAC **prefix**, not a full address:
  ```yaml
  apiVersion: v1alpha1
  kind: LinkAliasConfig
  name: net0
  selector:
    match: link.driver == "atlantic" && mac(link.permanent_addr).startsWith("00:30:93:12:")
  ```
  — `talos/cluster.yaml.j2:19-23`. Every node runs the identical selector text; it resolves
  differently per machine only because each physical node has exactly one NIC matching that prefix.
  **This does not transfer to our fleet as-is** — our `talos/talconfig.yaml` uses a full, distinct
  `hardwareAddr` per node (e.g. `70:70:fc:07:d0:7b` for `ser8a`), so we cannot copy onedr0p's
  single-shared-selector trick; we'd need either genuinely distinct prefixes per node (unlikely,
  same hardware SKU) or to keep the per-node file split (as they do for hostname/labels) and put the
  full MAC selector inside each node's own `nodes/<role>/<node>.yaml.j2`, which onedr0p's sample
  simply doesn't need to demonstrate.
- **VLAN IDs**: also shared/uniform, not per-node — `VLANConfig` documents for `bond0.70` and
  `bond0.90` sit in the shared `cluster.yaml.j2:38-51`, because (per onedr0p's fleet) every node
  carries the same VLANs.
- **Static VIP**: **onedr0p has no Talos-level VIP at all.** Grepping all of `talos/` for
  `vip`/`VirtualIP`/`kube-vip` returns nothing. Their control-plane API endpoint
  (`https://k8s.internal:6443`) is fronted instead by a Cilium `LoadBalancer` Service + BGP
  announcement to their router, documented in `bootstrap/README.md:29-52` (`CiliumLoadBalancerIPPool`
  + `CiliumBGPAdvertisement`/`CiliumBGPPeerConfig`/`CiliumBGPClusterConfig` under
  `kubernetes/apps/kube-system/cilium/app/networking.yaml`), not a `machine.network.interfaces[].vip`
  field. **This is a real gap for #1509**: our `talconfig.yaml` sets a per-node-identical static
  `vip: ip: "10.0.10.245"` on the bond interface (kube-vip-style, Talos-native), and onedr0p's
  reference repo simply doesn't have an example of that pattern to translate — they solved the same
  problem architecturally differently (BGP-announced Service VIP instead of a Talos machine-level
  VIP). #1509 will need to either find a real multi-doc example of a Talos-native VIP elsewhere, or
  decide whether to follow onedr0p's route (drop the Talos VIP, adopt Cilium LB + BGP) as a package
  deal with the config migration.
- **Per-node labels**: as covered in §2 — split across `KubeNodeConfig.labels` at three layers
  (shared / role / per-node file), the per-node piece living in each node's own
  `nodes/controlplane/<node>.yaml.j2`.

The role split (`controlplane.yaml.j2` vs. an as-yet-nonexistent `workers.yaml.j2`) is chosen by
directory (`talos/README.md:35-37`), and node file discovery uses that directory, not a `nodes:`
list in a config file — there is no registry file at all; adding a node means creating a new
`nodes/<role>/<hostname>.yaml.j2` file and nothing else.

## 5. Task runner recipes

`talos/mod.just` (invoked as `just talos <recipe>` via the root `.justfile:16-18` module import):

| Recipe | What it invokes |
|---|---|
| `apply-node <node>` | `just talos render-config <node> \| talosctl -n <node> apply-config -f /dev/stdin` — confirms via `[confirm(...)]` attribute first — `talos/mod.just:7-10` |
| `render-config <node>` | The three-layer `talosctl machineconfig patch` pipeline described in §1 — `talos/mod.just:26-34` |
| `download-image <version> [node]` | Looks up the schematic ID, `curl`s the metal ISO from `factory.talos.dev/image/<schematic>/<version>/metal-amd64.iso` — `talos/mod.just:12-18` |
| `reboot-node <node>` | `talosctl -n <node> reboot -m powercycle` — `talos/mod.just:20-23` |
| `reset-node <node> *args` | `talosctl -n <node> reset --system-labels-to-wipe STATE --system-labels-to-wipe EPHEMERAL --system-labels-to-wipe u-local-hostpath --graceful=false` — `talos/mod.just:36-39` |
| `shutdown-node <node> *args` | `talosctl -n <node> shutdown --force` — `talos/mod.just:41-44` |
| `upgrade-k8s <version>` | `talosctl -n <endpoint> upgrade-k8s --to <version>` (endpoint derived from `talosctl config info`) — `talos/mod.just:46-49` |
| `upgrade-node <node> *args` | `talosctl -n <node> upgrade -i "$(just talos machine-image <node>)" -m powercycle --timeout=10m` — `talos/mod.just:51-54` |
| `schematic-id`/`schematic-file`/`machine-image` (private) | Image Factory schematic resolution helpers used by the above — `talos/mod.just:56-72` |

Bootstrap (a separate `bootstrap/mod.just`, not under `talos/`) drives first-cluster-up via
`just bootstrap cluster`, whose stages are `nodes → k8s → kubeconfig → base → apps`
(`bootstrap/README.md:214-256`): stage 1 renders+applies each node's config exactly via
`render-config`/`apply-config --insecure`; stage 2 runs `talosctl bootstrap`; the rest is
kubeconfig fetch, bootstrap Secrets/CRDs, then `helmfile sync` to get Cilium/CoreDNS/cert-manager/
external-secrets/1Password Connect/flux-operator/flux-instance up, after which Flux takes over.
There is no separate "apply talhelper genconfig" step anywhere — `render-config`/`apply-node` *is*
the entire config-generation-and-push path, for both initial bootstrap and ongoing day-2 changes.

## Summary of key takeaways for #1508 / #1509

- **#1508** (document-kind structure): every one of our 6 legacy fields maps cleanly to a documented
  multi-doc kind with a real example except `admissionControl`, which onedr0p simply doesn't set —
  that's a genuine open question, not something this research resolves.
- **#1509** (secrets): no regeneration needed either way — talhelper and onedr0p's raw
  `op://` references both ultimately point at the same `talosctl gen secrets`-shaped field set;
  the only decision is where those existing values live (SOPS-in-repo vs. externalized secret
  store) and how they get injected into the new template layer.
- **#1509** (per-node templating): onedr0p's model is "one file per node," not a shared vars table —
  there's no ready-made Jinja-loop-over-a-nodes-array pattern to copy. Our full-MAC-per-node and
  static-Talos-VIP requirements don't have a direct analog in onedr0p's current repo state (their
  MAC match is a shared vendor prefix; their control-plane endpoint uses Cilium+BGP instead of a
  Talos VIP) — #1509 needs to design those two pieces from scratch rather than transplant them.
