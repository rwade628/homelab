---
status: accepted
---

# Omit `KubeAdmissionControlConfig` in the Talos multi-doc migration; ship zero admission-plugin configuration

`talos/patches/controller/cluster.yaml` currently deletes `cluster.apiServer.admissionControl`
outright (`$$patch: delete`), so no admission-plugin configuration (in particular no PodSecurity
policy) is supplied to kube-apiserver today. Migrating to Talos v1.14's multi-document config
kinds (tracked in [#1504](https://github.com/rwade628/homelab/issues/1504)) introduces
`KubeAdmissionControlConfig`, a new one-document-per-plugin kind, and `talosctl gen config`'s
default for fresh clusters emits one for `PodSecurity` (baseline/restricted enforcement). Reading
`internal/app/machined/pkg/controllers/k8s/control_plane_final.go` and
`render_config_static_pods.go` at the `v1.14.0` tag confirms Talos always hardcodes
`--enable-admission-plugins=NodeRestriction` and always renders an
`--admission-control-config-file`, regardless of whether any `KubeAdmissionControlConfig`
documents exist — presence/absence only changes what per-plugin configuration that file contains,
not whether kube-apiserver runs. So an absent document today is equivalent, byte-for-byte in
effect, to zero plugin configs, which is exactly our current state.

We chose to **not** add a `KubeAdmissionControlConfig` document, keeping the multi-doc migration
behavior-neutral. `onedr0p/home-ops` (the reference implementation this migration otherwise
follows) makes the same choice — no `KubeAdmissionControlConfig` document anywhere in their
`talos/` tree.

This is called out as its own ADR — rather than left as a silent omission — because a future
reader diffing the multi-doc config against `talosctl gen config`'s own defaults would reasonably
wonder why no `PodSecurity` admission plugin config was added, and because reversing this later
(adding baseline/restricted PodSecurity enforcement) is a real, cluster-wide security-posture
change, not a config-representation detail.

## Considered options

- **Add `talosctl gen config`'s default `PodSecurity` `KubeAdmissionControlConfig`
  (baseline/restricted).** Rejected for this migration: it would newly enforce Pod Security
  Standards across every namespace, a behavior change unrelated to and out of scope for a
  representation-only migration (see the map's hard constraint: "preserve current effective
  behavior exactly... this is about representation, not revisiting the platform choices").
  Worth revisiting as its own deliberate decision later.
- **Add an explicit no-op / empty marker document.** Rejected: `KubeAdmissionControlConfig` has no
  "disabled" or empty form — the kind is fundamentally one-document-per-plugin, so there is no
  document to add that means "no plugins." Omission is the only faithful representation of "zero
  plugin configs."

## Consequences

- Kube-apiserver continues running with only the built-in `NodeRestriction` plugin and Kubernetes'
  own compiled-in admission defaults — no PodSecurity policy is enforced cluster-wide, matching
  today.
- Adopting PodSecurity enforcement (or any other admission plugin config) later means adding a new
  `KubeAdmissionControlConfig` document as a standalone, deliberate change — not something that
  falls out of finishing this migration.
