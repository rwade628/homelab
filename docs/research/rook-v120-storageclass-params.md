# Which StorageClass parameter changes does Rook v1.20 actually require for `ceph-block`?

Research for issue [#1451](https://github.com/rwade628/homelab/issues/1451), a child of the
"Rook-Ceph v1.20 Migration Map" wayfinder issue [#1450](https://github.com/rwade628/homelab/issues/1450).

No `docs/research/` (or similar) convention existed in this repo before this file; it establishes
the location per the task instructions.

## TL;DR / direct recommendation for this cluster

**None of the three `ceph-block` StorageClass parameter differences from the onedr0p reference
cluster are actually caused or required by the Rook v1.20 / ceph-csi-operator migration.**
Rook's own v1.20.5 chart defaults, and upstream ceph-csi's own canonical example, still ship
both `controller-publish-secret-name/-namespace` **and** `csi.storage.k8s.io/fstype: ext4`
together, unchanged from years-old versions. onedr0p's specific values predate their v1.20
migration PRs entirely (see [Q1](#q1) and [Q2](#q2)) — they are a separate, optional feature
opt-in and a stylistic omission, not v1.20 requirements.

Recommendation:

1. **Do not touch `fstype`.** Leave `csi.storage.k8s.io/fstype: ext4` in place. It is optional,
   already the CSI default, has no interaction with the v1.20 CSI-driver-chart split, and
   removing it is a no-op at best — not worth the immutable-field churn described in
   [Q3](#q3).
2. **Adding `controller-publish-secret-name/-namespace` is optional, not required**, for a
   successful v1.20 upgrade. It only matters if this cluster wants to opt into two *specific*
   ceph-csi features that call the CSI `ControllerPublishVolume`/`ControllerUnpublishVolume`
   RPCs: (a) RBD non-graceful-node-shutdown fencing (auto-blocklisting a dead Talos node's RBD
   client so its PV can be safely rescheduled), and (b) ServiceAccount-based RBD mount
   restriction. Given this is a Talos cluster where a node can go hard-down, (a) is plausibly
   worth having — but it is a deliberate feature adoption decision, not a v1.20 migration
   blocker. Nothing breaks in the ceph-csi-operator controller-plugin/node-plugin split if it's
   left absent; `ControllerPublishVolume` simply isn't called for ordinary RWO attach/detach in
   Kubernetes' standard flow (see [Q1](#q1)).
3. **If/when the parameters *are* changed**, the safe path given `StorageClass.parameters`
   immutability (confirmed in Kubernetes' own source, [Q3](#q3)) is delete-then-recreate the
   `ceph-block` object with the same `name`/`provisioner`, **not** an in-place edit. This repo's
   own live `PersistentVolume` objects (inspected directly against the cluster, see
   [Q3](#q3)) prove that already-bound PVs do not re-read the StorageClass at all after
   provisioning — the relevant fields (`fsType`, `*SecretRef`, `volumeAttributes`) are copied
   once into `PV.spec.csi` at provision time by the CSI external-provisioner sidecar, and
   `PV.spec.storageClassName` is used only as a display/matching string thereafter. Recreating
   the SC object is safe for existing PVs; it only changes what *new* PVCs get. Existing PVs
   provisioned before the change will simply lack `controllerPublishSecretRef` unless
   recreated or backed by the ceph-csi `csi-config-map` fallback (see [Q1](#q1), "Solution 1").

---

## Q1: Is `controller-publish-secret-name`/`-namespace` required by the ceph-csi-operator model, or optional? What breaks without it? {#q1}

**Optional in general; required only for two specific opt-in features. Not tied to v1.20 at all.**

- Rook's v1.20 upgrade guide itself
  ([`Documentation/Upgrade/rook-upgrade.md` at tag `v1.20.5`](https://github.com/rook/rook/blob/v1.20.5/Documentation/Upgrade/rook-upgrade.md))
  says nothing about StorageClass parameters. Its only CSI-migration requirements are: install
  the new `ceph-csi-drivers` chart ("This `ceph-csi-drivers` chart must be installed, otherwise
  the CSI driver will be in a failed state due to missing service accounts"), and move CSI
  settings that used to live in the `rook-ceph` chart's `csi:` values block into the operator's
  `OperatorConfig`/`Driver` CRs instead. No StorageClass changes are mentioned.
- The `controller-publish-secret-name/-namespace` parameters are **not new** — they were already
  present in Rook's `rook-ceph-cluster` chart default `values.yaml` by `v1.18.5` (absent at
  `v1.17.6`, confirmed by diffing
  [`deploy/charts/rook-ceph-cluster/values.yaml`](https://github.com/rook/rook/blob/v1.20.5/deploy/charts/rook-ceph-cluster/values.yaml)
  across tags), i.e. roughly two years before v1.20. onedr0p's own `ceph-block` StorageClass
  values in `kubernetes/apps/rook-ceph/rook-ceph/cluster/helmrelease.yaml` already carried
  `controller-publish-secret-name/-namespace` in commits from 2023–2025, long before their v1.20
  migration PRs (#11548, #11552) — and neither of those PRs touches the StorageClass block at
  all. This is decisive: the parameter has nothing to do with the ceph-csi-operator migration.
- What it's actually *for*, per ceph-csi's own docs
  ([`docs/rbd/deploy.md`, "Kubernetes ServiceAccount Based Volume Access"](https://github.com/ceph/ceph-csi/blob/devel/docs/rbd/deploy.md)):
  > "This feature requires controller-publish-secret set in storageclass for newer PVCs. For
  > existing PVCs, the workaround mentioned [here](https://github.com/ceph/ceph-csi/blob/devel/docs/design/proposals/non-graceful-node-shutdown.md#workaround-for-older-pvs) can be used."
- The second, more homelab-relevant use is ceph-csi's
  [non-graceful node shutdown design doc](https://github.com/ceph/ceph-csi/blob/devel/docs/design/proposals/non-graceful-node-shutdown.md):
  when a node is tainted `node.kubernetes.io/out-of-service` (e.g. a dead/unresponsive Talos
  node), Kubernetes force-deletes pods and `VolumeAttachment`s and calls
  `ControllerUnpublishVolume` *without* the node ever running `NodeUnpublishVolume`/
  `NodeUnstageVolume`. ceph-csi uses that `ControllerUnpublishVolume` call to blocklist the
  dead node's RBD client (`ceph osd blocklist add <clientAddress> ...`), which is what makes it
  safe to reschedule the RWO volume elsewhere without corruption. That doc states verbatim:
  > "**Problem**: `ControllerPublishVolume()`/`ControllerUnpublishVolume()` requires
  > controller-publish-secret. The secret needed to access the Ceph cluster may be missing for
  > older PVs if the following parameters were not specified in their corresponding StorageClass
  > at the time of provisioning: `csi.storage.k8s.io/controller-publish-secret-name`,
  > `csi.storage.k8s.io/controller-publish-secret-namespace`."
  > "**Solution 1**: Fallback to default secrets if available in csi-config-map ConfigMap"
  > (a cluster-wide `controllerPublishSecretRef` in the `rook-ceph-csi-config` ConfigMap's
  > per-cluster JSON entry) covers PVs that don't have it set per-PV.
- **What concretely breaks without it**: nothing in ordinary operation. Standard Kubernetes RWO
  attach/detach for CSI drivers that don't advertise the `PUBLISH_UNPUBLISH_VOLUME` controller
  capability, or where the driver doesn't need it, doesn't call `ControllerPublishVolume` at all
  for basic provisioning/attach — provisioning, expansion, and normal mount/unmount work fine
  without this secret (confirmed empirically: this cluster's live PVs, which lack
  `controllerPublishSecretRef` entirely, are healthy and `Bound`, see [Q3](#q3)). What's missing
  is specifically the two RPC-driven features above: non-graceful-shutdown fencing and
  ServiceAccount mount restriction. Absent the secret, `ControllerPublishVolume`/
  `ControllerUnpublishVolume` calls for a PV lacking it will fail (per the doc's own framing of
  this as a "problem" requiring a workaround), meaning the auto-blocklist-on-dead-node fencing
  path won't run for that PV — a real but narrow gap, not a driver failure.
- ceph-csi-operator's own `ceph-csi-drivers` chart doc
  ([`docs/helm-charts/drivers-chart.md`](https://github.com/ceph/ceph-csi-operator/blob/main/docs/helm-charts/drivers-chart.md))
  confirms the chart can optionally template StorageClasses itself
  (`drivers.rbd.storageClasses`), pointing at ceph-csi's own canonical example
  (`https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/storageclass.yaml`) for available
  parameters — it does not mandate any particular parameter set, and this cluster doesn't use
  that chart-generated-StorageClass path anyway (the `ceph-block` SC here, like onedr0p's, is
  rendered by the `rook-ceph-cluster` chart's `cephBlockPools[].storageClass`, a path the CSI
  driver chart split doesn't touch at all).

## Q2: Why does `fstype: ext4` disappear in the v1.20/ceph-csi-operator model — deprecated, defaulted, or just optional either way? {#q2}

**It hasn't disappeared upstream at all — it's explicitly documented as optional and defaults to `ext4`, unchanged for years. Its absence in onedr0p's config is their own choice, unrelated to v1.20.**

- Rook's `rook-ceph-cluster` chart default `values.yaml`, checked at three tags spanning the
  v1.20 boundary — [`v1.16.4`](https://github.com/rook/rook/blob/v1.16.4/deploy/charts/rook-ceph-cluster/values.yaml),
  [`v1.19.9`](https://github.com/rook/rook/blob/v1.19.9/deploy/charts/rook-ceph-cluster/values.yaml),
  and [`v1.20.5`](https://github.com/rook/rook/blob/v1.20.5/deploy/charts/rook-ceph-cluster/values.yaml)
  — carries the **identical** comment and parameter in all three:
  > "Specify the filesystem type of the volume. If not specified, csi-provisioner will set
  > default as `ext4`. Note that `xfs` is not recommended due to potential deadlock in
  > hyperconverged settings where the volume is mounted on the same node as the osds."
  > `csi.storage.k8s.io/fstype: ext4`
  This is Rook's own recommended default at v1.20.5 — it is not deprecated or removed in the
  chart Rook itself ships for v1.20.
- Upstream ceph-csi's own canonical RBD StorageClass example
  ([`examples/rbd/storageclass.yaml`, `devel` branch](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/storageclass.yaml))
  marks it explicitly `(optional)`:
  > "(optional) Specify the filesystem type of the volume. If not specified, csi-provisioner
  > will set default as `ext4`."
  and also documents that `mkfsOptions` defaults themselves are keyed off this same setting
  (`ext4` → `-m0 -Enodiscard,lazy_itable_init=1,lazy_journal_init=1`), so setting it explicitly
  to `ext4` (as this cluster does) simply makes the implicit default explicit — it changes
  nothing.
- The `rook-ceph-cluster` chart's own StorageClass rendering template
  ([`templates/cephblockpool.yaml` at `v1.20.5`](https://github.com/rook/rook/blob/v1.20.5/deploy/charts/rook-ceph-cluster/templates/cephblockpool.yaml))
  only auto-injects `pool` and `clusterID` into `parameters`; every other key (`fstype`,
  `imageFeatures`, all the secret refs) comes straight from `$blockpool.storageClass.parameters`
  in values — the chart itself does not special-case, default, or strip `fstype` in v1.20 versus
  earlier versions. There is no chart-level mechanism that "moved" the default elsewhere.
- Conclusion: `fstype` did not become deprecated, implied-elsewhere, or newly-defaulted by the
  v1.20/ceph-csi-operator split. It was already optional and already defaulted to `ext4` in
  every version checked. onedr0p simply chose to omit an already-redundant explicit value; this
  cluster's explicit `ext4` is equally correct and safe to leave as-is.

## Q3: Given `StorageClass.parameters` immutability and already-bound PVs, what's the safe migration path? {#q3}

**Confirmed immutable at the Kubernetes API validation layer. Confirmed empirically (against this cluster's own live PVs) that bound PVs do not re-read the StorageClass after provisioning — delete+recreate of the SC object (same name/provisioner) is safe for existing PVs.**

- **Immutability, at the source**: Kubernetes' own API validation code,
  [`pkg/apis/storage/validation/validation.go`, `ValidateStorageClassUpdate`](https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/storage/validation/validation.go)
  (as of `master`, current):
  ```go
  func ValidateStorageClassUpdate(storageClass, oldStorageClass *storage.StorageClass) field.ErrorList {
      allErrs := apivalidation.ValidateObjectMetaUpdate(...)
      allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(storageClass.Parameters, oldStorageClass.Parameters, field.NewPath("parameters"))...)
      allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(storageClass.Provisioner, oldStorageClass.Provisioner, field.NewPath("provisioner"))...)
      allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(storageClass.ReclaimPolicy, oldStorageClass.ReclaimPolicy, field.NewPath("reclaimPolicy"))...)
      allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(storageClass.VolumeBindingMode, oldStorageClass.VolumeBindingMode, field.NewPath("volumeBindingMode"))...)
      return allErrs
  }
  ```
  `parameters`, `provisioner`, `reclaimPolicy`, and `volumeBindingMode` are all enforced
  immutable by the apiserver itself — an in-place `kubectl apply`/`edit` changing `parameters`
  will be rejected with a `field is immutable` error. (`mountOptions`, `allowedTopologies`, and
  metadata like `labels`/`annotations` are not in this list and remain editable.)
- **PVs don't re-read the StorageClass after provisioning — confirmed against this cluster's own
  live objects**, not just theory. `kubectl get pv pvc-096ff90d-bb38-4140-8b21-1a885c47662b -o yaml`
  on this cluster shows:
  ```yaml
  spec:
    csi:
      controllerExpandSecretRef: {name: rook-csi-rbd-provisioner, namespace: rook-ceph}
      driver: rook-ceph.rbd.csi.ceph.com
      fsType: ext4
      nodeStageSecretRef: {name: rook-csi-rbd-node, namespace: rook-ceph}
      volumeAttributes:
        clusterID: rook-ceph
        imageFeatures: layering,fast-diff,object-map,deep-flatten,exclusive-lock
        imageFormat: "2"
        pool: ceph-blockpool
        ...
    storageClassName: ceph-block
  ```
  Every parameter that came from the StorageClass at provisioning time (`fsType`, the various
  `*SecretRef`s, `imageFeatures`, `imageFormat`, `pool`) is **copied directly into the PV's own
  `spec.csi` block** by the CSI `external-provisioner` sidecar at `CreateVolume` time.
  `spec.storageClassName` afterward is just a string label (used for `kubectl get pv` display,
  PVC-to-PV matching during binding, and by some controllers for informational purposes) — it is
  not a live reference the CSI driver or kubelet dereferences on every mount. Note also that this
  PV already lacks `controllerPublishSecretRef` (because it predates any `controller-publish-secret`
  parameter on this cluster's SC) yet is healthy and `Bound`, which is itself direct evidence
  supporting [Q1](#q1)'s "optional, narrow blast radius" conclusion.
- **Practical implication**: because (a) `parameters` can't be edited in place and (b) already-
  provisioned PVs don't consult the SC object again, the standard/only path to change
  `ceph-block`'s parameters is **delete the StorageClass object and recreate it** with the same
  `metadata.name: ceph-block` and same `provisioner: rook-ceph.rbd.csi.ceph.com`. This is safe
  for existing PVs specifically because of the copy-at-provision-time behavior shown above — a
  bound PV has no live dependency on the SC object continuing to exist with matching parameters.
  It only affects:
  - **New PVCs** provisioned after the recreate — they'll get the new parameters baked into
    their new PVs, as expected.
  - **Volume expansion** (`ControllerExpandVolume`) and any other secret-driven RPC called
    against an *existing* PV — those already read the secret ref baked into that PV's own
    `spec.csi.controllerExpandSecretRef` etc., not the live SC, so they're unaffected either.
  - Anything that specifically depends on the SC object's continued existence by UID (e.g. an
    `ownerReference` pointing at the old SC) — none exists in this setup; PVCs/PVs reference SCs
    by name only (`storageClassName: string`), not by UID or object reference, per the
    PersistentVolume/StorageClass API shape used throughout (confirmed via the live PV dump
    above and the `rook-ceph-cluster` chart template).
  - `isDefault` / `storageclass.kubernetes.io/is-default-class` annotation churn: recreating
    the object briefly removes the default annotation from existence; if any automation
    provisions PVCs with no explicit `storageClassName` during that gap it would fail to bind
    until the new SC lands. Practically instantaneous in a GitOps apply, but worth doing as a
    single atomic `kubectl replace --force` (delete+recreate in one step) or a Flux
    reconciliation rather than a manual delete-then-wait-then-create, to minimize the window.
  - No source found (Kubernetes docs, Rook docs, or ceph-csi docs) that documents an alternative,
    "cleaner" migration path for changing SC parameters in place — Kubernetes' own
    [StorageClasses concept doc](https://kubernetes.io/docs/concepts/storage/storage-classes/)
    doesn't address the immutability/recreate question at all (checked directly; it only
    describes create-time semantics), which is why the source-code citation above and this
    cluster's own live-PV evidence are the two hardest data points available, in the absence of
    an authoritative doc.

---

## Sources consulted

| Source | What it established |
|---|---|
| [Rook v1.20 upgrade guide, `v1.20.5`](https://github.com/rook/rook/blob/v1.20.5/Documentation/Upgrade/rook-upgrade.md) | No StorageClass parameter changes required by the migration; only the new `ceph-csi-drivers` chart + CR config relocation. |
| [`rook-ceph-cluster` chart `values.yaml`: `v1.16.4`](https://github.com/rook/rook/blob/v1.16.4/deploy/charts/rook-ceph-cluster/values.yaml), [`v1.19.9`](https://github.com/rook/rook/blob/v1.19.9/deploy/charts/rook-ceph-cluster/values.yaml), [`v1.20.5`](https://github.com/rook/rook/blob/v1.20.5/deploy/charts/rook-ceph-cluster/values.yaml) | `fstype: ext4` present and identically commented at all three tags; `controller-publish-secret-*` added between `v1.17.6` and `v1.18.5`, unrelated to v1.20. |
| [`rook-ceph-cluster` chart `templates/cephblockpool.yaml`, `v1.20.5`](https://github.com/rook/rook/blob/v1.20.5/deploy/charts/rook-ceph-cluster/templates/cephblockpool.yaml) | Chart only auto-injects `pool`/`clusterID`; all other SC parameters (fstype, secrets) are pure values passthrough, unchanged mechanism in v1.20. |
| [ceph-csi `docs/rbd/deploy.md`, `devel`](https://github.com/ceph/ceph-csi/blob/devel/docs/rbd/deploy.md) | `controller-publish-secret` requirement is specifically for the optional "Kubernetes ServiceAccount Based Volume Access" feature. |
| [ceph-csi `docs/design/proposals/non-graceful-node-shutdown.md`, `devel`](https://github.com/ceph/ceph-csi/blob/devel/docs/design/proposals/non-graceful-node-shutdown.md) | `controller-publish-secret` is what `ControllerPublishVolume`/`ControllerUnpublishVolume` need for dead-node RBD client blocklisting; documents fallback via `csi-config-map` for PVs that lack it. |
| [ceph-csi `examples/rbd/storageclass.yaml`, `devel`](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/storageclass.yaml) | Canonical upstream example: `fstype` explicitly marked `(optional)`, defaults to `ext4`; `controller-publish-secret` included by default but not marked required. |
| [ceph-csi-operator `docs/helm-charts/drivers-chart.md`](https://github.com/ceph/ceph-csi-operator/blob/main/docs/helm-charts/drivers-chart.md) | The `ceph-csi-drivers` chart can optionally template StorageClasses itself, pointing at the same ceph-csi example above; doesn't mandate parameters; this cluster doesn't use that path. |
| [Kubernetes `pkg/apis/storage/validation/validation.go`, `master`](https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/storage/validation/validation.go) | `ValidateStorageClassUpdate` — authoritative confirmation that `parameters`, `provisioner`, `reclaimPolicy`, `volumeBindingMode` are immutable at the API layer. |
| This cluster's live `PersistentVolume` (`pvc-096ff90d-...`, `ceph-block`, via `kubectl get pv ... -o yaml`) | Direct empirical proof that provisioned-time SC parameters are copied into `PV.spec.csi.*` once and never re-read; `storageClassName` is a name-only reference thereafter; this specific PV already lacks `controllerPublishSecretRef` and is healthy/`Bound`. |
| [Kubernetes StorageClasses concept doc](https://kubernetes.io/docs/concepts/storage/storage-classes/) | Checked directly — does not address immutability or delete/recreate semantics; confirms parameters doc is otherwise create-time-only in scope. |
| [onedr0p/home-ops PR #11548](https://github.com/onedr0p/home-ops/pull/11548) | Secondary/supporting: v1.20.5 CSI-driver-chart migration; does **not** touch `ceph-block` StorageClass parameters. |
| [onedr0p/home-ops PR #11552](https://github.com/onedr0p/home-ops/pull/11552) | Secondary/supporting: Ceph Squid→Tentacle bump + cephx key rotation; also does **not** touch StorageClass parameters. |
| onedr0p/home-ops `kubernetes/apps/rook-ceph/rook-ceph/cluster/helmrelease.yaml` (current `main`, and commit history back to 2023) | The `controller-publish-secret`/no-`fstype` StorageClass shape predates both v1.20 PRs by roughly two years — confirms it's unrelated to the v1.20 migration. |
