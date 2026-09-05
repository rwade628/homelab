# Talos multi-doc migration spec: talhelper → `gen config` + `machineconfig patch` + `just`

Consolidated, diff-ready spec for
[Map: Migrate Talos machine config off talhelper to multi-doc kinds (#1504)](https://github.com/rwade628/homelab/issues/1504),
resolving its final ticket,
[Task: Write final ADR + consolidated diff-ready migration spec (#1511)](https://github.com/rwade628/homelab/issues/1511).
Decision rationale lives in
[ADR-0013](adr/0013-drop-talhelper-for-gen-config-machineconfig-patch-just.md) (this document is
the "how", ADR-0013 is the "why"). Field→kind mapping came from
[#1508](https://github.com/rwade628/homelab/issues/1508); mechanics (secrets/templating/layout/
task-runner) from [#1509](https://github.com/rwade628/homelab/issues/1509); schema grounding from
[#1506](https://github.com/rwade628/homelab/issues/1506); live-drift audit from
[#1507](https://github.com/rwade628/homelab/issues/1507); reference implementation from
[#1505](https://github.com/rwade628/homelab/issues/1505).

**This spec is written for a human to execute by hand.** Per the map's destination, no agent
applies any of this to the live cluster — `talosctl apply-config` and node sequencing are the
human-executed last mile. Every step below that touches the live cluster is marked accordingly.

## Target file tree

```
talos/
├── clusterconfig/
│   ├── .gitignore          (unchanged)
│   └── talosconfig         (unchanged — talosctl client config)
├── cluster.yaml            (new — shared across all 3 nodes)
├── controlplane.yaml       (new — control-plane-only; currently all 3 nodes)
├── mod.just                (new)
├── nodes/
│   ├── ser8a.yaml           (new)
│   ├── ser8b.yaml           (new)
│   └── ser8c.yaml           (new)
├── talsecret.sops.yaml     (unchanged — still the secrets bundle, still SOPS-encrypted)
└── versions.yaml           (new — replaces talenv.yaml)

.justfile                   (new, repo root — one line: `mod talos`)
```

Deleted: `talos/talconfig.yaml`, `talos/talenv.yaml`, `talos/patches/` (including its `README.md`),
`.taskfiles/talos/Taskfile.yaml`. Edited: root `Taskfile.yaml` (remove the `talos:
.taskfiles/talos` include line).

## File contents

### `talos/versions.yaml` (new — replaces `talenv.yaml`)

Keeps the exact `# renovate: datasource=... depName=...` comment convention `talconfig.yaml` used,
so Renovate's existing custom managers (`home-operations/renovate-presets`'s generic
annotated-dependency regex, and its `talosFactory` preset's `talosVersion:` regex — both already
active via `.renovaterc.json5`, neither needs reconfiguring) keep bumping these without any
Renovate-side change.

```yaml
# renovate: datasource=docker depName=ghcr.io/siderolabs/installer
talosVersion: v1.14.0
# renovate: datasource=docker depName=ghcr.io/siderolabs/kubelet
kubernetesVersion: v1.36.1
# schematic ID is stable — bump manually only if the Image Factory schematic (system
# extensions/kernel args) changes; not tracked by the renovate managers above
schematicId: 4b82055dec8c9571600a4ceeebddbe33b8ca9ec9a9aaefa64853b2bab3b76993
```

### `talos/cluster.yaml` (new — applied to every node)

Everything identical across `ser8a`/`ser8b`/`ser8c` today: bond/VLAN/VIP network config, disk
selector, sysctls, udev rules, files, time servers, `hostDNS`, the region label, kubelet
extraArgs/config, `nodeIP.validSubnets`, `SecurityProfileConfig`, and the two `cluster:` fields
with no multi-doc kind yet (`etcd.*`, `network.*`).

```yaml
cluster:
  network:
    cni:
      name: none
    podSubnets:
      - 10.69.0.0/16
    serviceSubnets:
      - 10.96.0.0/16
  etcd:
    advertisedSubnets:
      - 10.0.10.0/24
    extraArgs:
      listen-metrics-urls: http://0.0.0.0:2381
machine:
  install:
    diskSelector:
      model: CT1000P3PSSD8
  time:
    disabled: false
    servers:
      - 162.159.200.1
      - 162.159.200.123
  sysctls:
    fs.inotify.max_user_instances: "8192"
    fs.inotify.max_user_watches: "1048576"
    net.core.default_qdisc: fq
    net.core.rmem_max: "67108864"
    net.core.wmem_max: "67108864"
    net.ipv4.neigh.default.gc_thresh1: "4096"
    net.ipv4.neigh.default.gc_thresh2: "8192"
    net.ipv4.neigh.default.gc_thresh3: "16384"
    net.ipv4.ping_group_range: 0 2147483647
    net.ipv4.tcp_congestion_control: bbr
    net.ipv4.tcp_fastopen: "3"
    net.ipv4.tcp_mtu_probing: "1"
    net.ipv4.tcp_notsent_lowat: "131072"
    net.ipv4.tcp_rmem: 4096 131072 67108864
    net.ipv4.tcp_slow_start_after_idle: "0"
    net.ipv4.tcp_window_scaling: "1"
    net.ipv4.tcp_wmem: 4096 65536 67108864
    sunrpc.tcp_max_slot_table_entries: "128"
    sunrpc.tcp_slot_table_entries: "128"
    user.max_user_namespaces: "11255"
  udev:
    rules:
      - SUBSYSTEM=="drm", KERNEL=="card*", GROUP="44", MODE="0660"
      - SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"
  files:
    - op: create
      path: /etc/cri/conf.d/20-customization.part
      content: |
        [plugins."io.containerd.cri.v1.images"]
          discard_unpacked_layers = false
        [plugins."io.containerd.cri.v1.runtime"]
          device_ownership_from_security_context = true
    - op: overwrite
      path: /etc/nfsmount.conf
      permissions: 0o644
      content: |
        [ NFSMount_Global_Options ]
        nfsvers=4.2
        hard=True
        nconnect=8
        noatime=True
        rsize=1048576
        wsize=1048576
  features:
    hostDNS:
      enabled: true
      resolveMemberNames: true
      forwardKubeDNSToHost: true # Requires Cilium `bpf.masquerade: false`
---
apiVersion: v1alpha1
kind: BondConfig
name: bond0
links:
  - bond0-m0
bondMode: active-backup
mtu: 9000
---
apiVersion: v1alpha1
kind: DHCPv4Config
name: bond0
---
apiVersion: v1alpha1
kind: Layer2VIPConfig
name: 10.0.10.245
link: bond0
---
apiVersion: v1alpha1
kind: VLANConfig
name: bond0.30
vlanID: 30
parent: bond0
mtu: 9000
---
apiVersion: v1alpha1
kind: VLANConfig
name: bond0.50
vlanID: 50
parent: bond0
mtu: 9000
---
apiVersion: v1alpha1
kind: VLANConfig
name: bond0.99
vlanID: 99
parent: bond0
mtu: 9000
---
apiVersion: v1alpha1
kind: KubeNodeConfig
labels:
  topology.kubernetes.io/region: main
nodeIP:
  validSubnets:
    - 10.0.10.0/24
---
apiVersion: v1alpha1
kind: KubeletConfig
extraArgs:
  allowed-unsafe-sysctls: net.ipv6.*
config:
  serializeImagePulls: false
  imageMaximumGCAge: 60m
---
apiVersion: v1alpha1
kind: SecurityProfileConfig
workloadIsolation: false
```

### `talos/controlplane.yaml` (new — control-plane-only; currently every node)

```yaml
machine:
  features:
    kubernetesTalosAPIAccess:
      enabled: true
      allowedRoles:
        - os:admin
      allowedKubernetesNamespaces:
        - system-upgrade
---
apiVersion: v1alpha1
kind: KubeNodeConfig
labels:
  node-role.kubernetes.io/control-plane: ""
# taints intentionally omitted: this cluster's current `allowSchedulingOnControlPlanes: true`
# is exactly "control-plane label present, matching NoSchedule taint absent" — there is no
# boolean field in the multi-doc model, only presence/absence of that one taint entry.
---
apiVersion: v1alpha1
kind: KubeAPIServerConfig
extraArgs:
  # https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/
  enable-aggregator-routing: true
  feature-gates: ImageVolume=true,MutatingAdmissionPolicy=true
  runtime-config: admissionregistration.k8s.io/v1beta1=true
certExtraSANs:
  - 127.0.0.1
  - 10.0.10.245
---
apiVersion: v1alpha1
kind: KubeControllerManagerConfig
extraArgs:
  bind-address: 0.0.0.0
---
apiVersion: v1alpha1
kind: KubeSchedulerConfig
extraArgs:
  bind-address: 0.0.0.0
---
apiVersion: v1alpha1
kind: KubeProxyConfig
enabled: false
---
apiVersion: v1alpha1
kind: KubeCoreDNSConfig
enabled: false
```

No `KubeAdmissionControlConfig` document — deliberate, see
[ADR-0012](adr/0012-omit-kubeadmissioncontrolconfig-in-talos-multidoc-migration.md).

### `talos/nodes/ser8a.yaml` / `ser8b.yaml` / `ser8c.yaml` (new — per-node)

Only what's genuinely per-node: hostname, the exact full-MAC link selector, and the
hardware-derived labels from [ADR-0003](adr/0003-reject-amd-gpu-operator-for-integrated-graphics.md)
(identical values today, but a per-node fact, not a cluster-wide one — a future 4th node without
this GPU simply omits them).

`talos/nodes/ser8a.yaml`:

```yaml
apiVersion: v1alpha1
kind: HostnameConfig
hostname: ser8a
---
apiVersion: v1alpha1
kind: LinkAliasConfig
name: bond0-m0
selector:
  match: glob("70:70:fc:07:d0:7b", mac(link.hardware_addr))
---
apiVersion: v1alpha1
kind: KubeNodeConfig
labels:
  feature.node.kubernetes.io/amd-gpu: "true"
  topology.kubernetes.io/zone: m
```

`talos/nodes/ser8b.yaml` — identical except:

```yaml
kind: HostnameConfig
hostname: ser8b
---
# LinkAliasConfig match: glob("70:70:fc:07:d2:21", mac(link.hardware_addr))
```

`talos/nodes/ser8c.yaml` — identical except:

```yaml
kind: HostnameConfig
hostname: ser8c
---
# LinkAliasConfig match: glob("70:70:fc:07:d0:d4", mac(link.hardware_addr))
```

### `talos/mod.just` (new)

Recipes run with `talos/` as the working directory (just's module default), so paths below are
relative to `talos/`. Replaces `.taskfiles/talos/Taskfile.yaml` 1:1 except `reset`'s node list is
now literal (talhelper derived it from `talconfig.yaml`'s `nodes:` array; there's no equivalent
registry file anymore).

```just
# talos/mod.just — invoked as `just talos <recipe>` via the root .justfile's `mod talos`

export TALOSCONFIG := "clusterconfig/talosconfig"

endpoint := "https://10.0.10.245:6443"
hostnames := "ser8a ser8b ser8c"

# Render a node's full machine config to stdout
[private]
render hostname:
    #!/usr/bin/env bash
    set -euo pipefail
    talos_version=$(yq '.talosVersion' versions.yaml)
    kubernetes_version=$(yq '.kubernetesVersion' versions.yaml)
    schematic=$(yq '.schematicId' versions.yaml)
    talosctl gen config homelab {{endpoint}} \
      --with-secrets <(sops -d talsecret.sops.yaml) \
      --kubernetes-version "${kubernetes_version}" \
      --install-image "factory.talos.dev/installer/${schematic}:${talos_version}" \
      --additional-sans 127.0.0.1,10.0.10.245 \
      --output-types controlplane -o - \
    | talosctl machineconfig patch /dev/stdin \
        -p @cluster.yaml \
        -p @controlplane.yaml \
        -p @nodes/{{hostname}}.yaml

# Diff a node's rendered config against its live running config (secrets/generated-only fields noisy — review by eye)
diff hostname:
    diff <(just talos render {{hostname}}) <(talosctl -n {{hostname}} get machineconfig -o yaml)

# Apply a node's rendered config. mode: no-reboot|auto|reboot
apply-config hostname mode="no-reboot":
    just talos render {{hostname}} | talosctl apply-config --mode={{mode}} --nodes {{hostname}} --file /dev/stdin

# Upgrade Talos on a single node. rollout=true skips the Flux suspend/resume (caller already did it)
upgrade-node hostname rollout="false":
    #!/usr/bin/env bash
    set -euo pipefail
    talos_version=$(yq '.talosVersion' versions.yaml)
    [ "{{rollout}}" = "true" ] || flux --namespace flux-system suspend kustomization --all
    schematic=$(kubectl get node {{hostname}} --output=jsonpath='{.metadata.annotations.extensions\.talos\.dev/schematic}')
    talosctl --nodes {{hostname}} upgrade --image="factory.talos.dev/installer/${schematic}:${talos_version}" --timeout=10m
    talosctl --nodes {{hostname}} health --wait-timeout=10m --server=false
    [ "{{rollout}}" = "true" ] || flux --namespace flux-system resume kustomization --all

# Upgrade Talos across the whole cluster, one node at a time
upgrade-cluster:
    #!/usr/bin/env bash
    set -euo pipefail
    flux --namespace flux-system suspend kustomization --all
    for hostname in {{hostnames}}; do
      just talos upgrade-node "$hostname" true
    done
    flux --namespace flux-system resume kustomization --all

# Upgrade the Kubernetes version
upgrade-k8s version:
    talosctl --nodes $(talosctl config info --output json | jq --raw-output '.endpoints[]' | shuf -n 1) upgrade-k8s --to {{version}}

# Reset all nodes back to maintenance mode (destructive)
reset *args:
    #!/usr/bin/env bash
    set -euo pipefail
    for hostname in {{hostnames}}; do
      talosctl --nodes "$hostname" reset --reboot {{args}} --graceful=false --wait=false
    done
```

### `.justfile` (new, repo root)

```just
mod talos
```

## What gets deleted

- `talos/talconfig.yaml`
- `talos/talenv.yaml` (replaced by `talos/versions.yaml`)
- `talos/patches/` (all of `controller/`, `global/`, and `README.md`)
- `.taskfiles/talos/Taskfile.yaml`
- The `talos: .taskfiles/talos` line in root `Taskfile.yaml`'s `includes:` block

Not deleted (out of scope — see [ADR-0013](adr/0013-drop-talhelper-for-gen-config-machineconfig-patch-just.md)'s
Consequences): `.taskfiles/bootstrap/Taskfile.yaml`'s talhelper calls (operates on the separate
`kubernetes/bootstrap/talos/` scaffolding tree, not this migration's target), and
`.taskfiles/workstation/Taskfile.yaml`/`Brewfile`/`Archfile`'s talhelper tool-install entries.

## Validation / cutover sequence

3 nodes, all control-plane, no staging environment — every step below is read-only or dry-run
until the explicit apply step, and applies one node at a time.

1. **Generate + diff, all 3 nodes, no cluster contact required for generation:**
   ```sh
   just talos diff ser8a
   just talos diff ser8b
   just talos diff ser8c
   ```
   Expect noise from generated-only/non-configurable fields (`machine.features.diskQuotaSupport`,
   `kubePrism`, `cluster.discovery`, `cluster.apiServer.auditPolicy`, cert/token values) — same
   noise class [#1507](https://github.com/rwade628/homelab/issues/1507) already characterized as
   safe to ignore. Look specifically for:
   - Any legacy `cluster.apiServer`/`controllerManager`/`scheduler`/`proxy`/`coreDNS` block
     persisting **with non-default values** alongside the new `Kube*Config` documents — would mean
     the base's auto-populated legacy block and our multi-doc patch disagree. If found, add an
     explicit `$$patch: delete` for that legacy sub-key in `cluster.yaml`/`controlplane.yaml`
     alongside the multi-doc addition (the same fix class #1507 diagnosed for `admissionControl`).
   - Duplicate or conflicting `certExtraSANs`/`certSANs` entries between the `--additional-sans`
     flag's output and the explicit `KubeAPIServerConfig.certExtraSANs` patch above — cosmetic if
     just a duplicate list entry, worth trimming one side if so.
   - `KubeNodeConfig` merging correctly across all 3 patch layers (region label from `cluster.yaml`
     + control-plane label from `controlplane.yaml` + GPU/zone labels from the per-node file all
     present in one final document, no layer silently overwriting another).
2. **Dry-run on one node first** (`ser8a`):
   ```sh
   just talos apply-config ser8a --mode=no-reboot
   ```
   Actually run this as a genuine dry run first — `talosctl apply-config --dry-run --mode=no-reboot
   --nodes ser8a --file <(just talos render ser8a)` — and inspect the reported diff before dropping
   `--dry-run`. **Live-cluster-touching from here on.**
3. **Apply to `ser8a`** (`no-reboot` mode — none of these changes touch `install.image`/disk, so a
   reboot shouldn't be required; if Talos reports a pending reboot-required change, use `--mode=reboot`
   next time instead of forcing one). Watch `talosctl -n ser8a get machineconfig -o yaml` and
   `kubectl get nodes` / `flux get ks -A` afterward for a beat before moving on.
4. **Repeat step 2–3 for `ser8b`, then `ser8c`**, one at a time, confirming cluster health
   (`talosctl health --server=false`, `kubectl get nodes`, `flux get ks -A`) between each — same
   one-node-at-a-time caution this repo's existing `upgrade-cluster` task already uses for Talos
   version upgrades, just applied to a config-only change instead.
5. **Only after all 3 nodes are confirmed on the new config**: delete the files listed above, add
   the new ones, update `CLAUDE.md`'s repo-layout description of `talos/` (currently says
   "`talconfig.yaml` (talhelper), `talenv.yaml`"), and commit.

## Deliberately unresolved / left for the diff step

This spec is grounded in the field-mapping and schema research from #1506/#1507/#1508, but two
specifics can only be confirmed by actually generating a config against this cluster's live
secrets bundle (not done as part of writing this spec — decrypting `talsecret.sops.yaml` outside
the human-executed apply flow was declined for this session):

- Whether `talosctl gen config --additional-sans` populates `KubeAPIServerConfig.certExtraSANs`
  automatically (in which case the explicit patch above is redundant, not wrong) or only the
  legacy `machine.certSANs`/`cluster.apiServer.certSANs` fields (in which case it's required).
- Whether Talos v1.14's default base leaves the legacy `cluster.apiServer`/`controllerManager`/
  `scheduler`/`proxy`/`coreDNS` blocks populated-but-inert once the corresponding multi-doc
  document exists, or whether they need explicit deletion — both are addressed by the diff-step
  guidance above, whichever way it lands.
