---
status: accepted
---

# Hand-author the Talos base config instead of `talosctl gen config`; source secrets from `talsecret.sops.yaml`

`talos/mod.just`'s `render` recipe (introduced in [ADR-0013](0013-drop-talhelper-for-gen-config-machineconfig-patch-just.md))
piped `talosctl gen config`'s freshly-generated base into `machineconfig patch`. Because `gen
config` regenerates that base from scratch on every render — reflecting whatever defaults *that*
Talos version's CLI currently bakes in — every Talos upgrade risks silently reintroducing a
default that conflicts with this cluster's desired end state, discoverable only by diffing the
rendered output after the fact. One session surfaced two: `KubeAdmissionControlConfig`/
`PodSecurity` (already documented, [ADR-0012](0012-omit-kubeadmissioncontrolconfig-in-talos-multidoc-migration.md))
and `machine.features.kubePrism` (found while adding an explicit `KubePrismConfig` document).
Reading `generate/kubernetes.go` and `generate/init.go` at the v1.14.0 tag confirms both — along
with the control-plane taint/label and the default Flannel CNI proposal — are artifacts of
`gen config`'s CLI-time defaulting, not anything `machined` enforces at runtime; skip the
generation step and none of them are ever introduced.

We switched `cluster.yaml`/`controlplane.yaml` to a hand-authored base — modeled directly on
`onedr0p/home-ops`'s `cluster.yaml.j2`/`controlplane.yaml.j2`, this repo's existing reference
implementation per ADR-0013 — with the machine/cluster CA certs, tokens, and cluster ID/secret
templated in as `$VAR` placeholders, substituted from fields already present in
`talsecret.sops.yaml` (a `talosctl gen secrets`-shaped bundle; no new secret material needed,
structure confirmed directly against the decrypted file). `mod.just`'s `render` recipe decrypts
that bundle once, exports each field as a named shell variable, and substitutes via **`yq`'s
built-in `envsubst` operator** (`yq e '(.. | select(tag == "!!str")) |= envsubst'`) rather than GNU
gettext's `envsubst` binary — `yq` is already a hard dependency of this recipe (and the rest of
this repo's tooling), and the workstation doesn't have gettext's `envsubst` installed, so this adds
no new dependency at all. The substituted output is written to a real temp file with a `.yaml`
suffix before being handed to `machineconfig patch` — a bare process-substitution fd
(`/dev/fd/N`) has no extension, and `talosctl` needs one to tell a full-document patch from a
JSON6902 patch (confirmed by hitting `"JSON6902 patches are not supported for multi-document
machine configuration"` when first tried via `-p @<(...)`).

## Considered options

- **Keep `talosctl gen config`, treat each new conflict as a one-off `$patch: delete`.** This is
  what ADR-0013 chose and what this repo did until now. Rejected on reconsideration: two conflicts
  surfacing across a single Talos version is enough to call this an ongoing, recurring cost rather
  than a one-time migration wrinkle — every future Talos upgrade re-exposes the same risk with no
  way to know in advance which field will collide next.
- **Externalize secrets to 1Password like `onedr0p/home-ops`, for closer parity.** Rejected for the
  same reason ADR-0013 rejected it: no 1Password Connect/`op` CLI wiring exists in this repo, and
  adding one is a real new dependency this change doesn't need — `talsecret.sops.yaml` already
  contains every field a hand-authored base requires.
- **Use a templating engine (`makejinja`, already used by `bootstrap/`) instead of variable
  substitution.** Rejected: ADR-0013 deliberately dropped templating from `talos/` entirely to keep
  it simple after leaving talhelper. Reintroducing a templating engine and its own delimiter
  convention for the sole purpose of splicing secret values into otherwise-static files is more
  machinery than the job needs — especially once GNU gettext's `envsubst` turned out not to be
  installed on the workstation either; `yq`'s built-in `envsubst` operator does the same job with a
  tool this recipe already depends on, adding nothing new.

## Consequences

- `mod.just`'s `render` recipe no longer calls `talosctl gen config`. It decrypts
  `talsecret.sops.yaml` once, exports the fields below as shell variables, and pipes `cluster.yaml`
  through `envsubst` (explicit variable list) before handing it to `machineconfig patch` as the
  base document — `controlplane.yaml` and `nodes/<hostname>.yaml` remain layered on top exactly as
  before.

  | Variable | Bundle path |
  |---|---|
  | `MACHINE_CA_CRT` / `MACHINE_CA_KEY` | `.certs.os.crt` / `.certs.os.key` |
  | `MACHINE_TOKEN` | `.trustdinfo.token` |
  | `CLUSTER_CA_CRT` / `CLUSTER_CA_KEY` | `.certs.k8s.crt` / `.certs.k8s.key` |
  | `CLUSTER_AGGREGATOR_CA_CRT` / `_KEY` | `.certs.k8saggregator.crt` / `.key` |
  | `CLUSTER_ETCD_CA_CRT` / `_KEY` | `.certs.etcd.crt` / `.key` |
  | `CLUSTER_SERVICEACCOUNT_KEY` | `.certs.k8sserviceaccount.key` |
  | `CLUSTER_TOKEN` | `.secrets.bootstraptoken` |
  | `CLUSTER_ID` / `CLUSTER_SECRET` | `.cluster.id` / `.cluster.secret` |
  | `CLUSTER_SECRETBOX_ENCRYPTION_SECRET` | `.secrets.secretboxencryptionsecret` |

- **Three existing `$patch: delete` directives are removed as dead weight**: `KubeFlannelCNIConfig`
  (cluster.yaml), the control-plane taint + `exclude-from-external-load-balancers` label deletes,
  and `KubeAdmissionControlConfig`/`PodSecurity` (controlplane.yaml) — none of the defaults they
  cancelled are ever introduced without `gen config` calling `generate/kubernetes.go`/
  `generate/init.go`. Flannel CNI specifically: `KubeFlannelCNIConfig`'s mere document *presence*
  is what deploys it (there's no `enabled: false` field on that document — confirmed reading
  `pkg/machinery/config/types/k8s/flannel.go`), so simply never emitting the document reaches "no
  Flannel" with no delete needed at all.
- **Deviated from `home-ops` on one point, after it failed validation**: `home-ops`'s
  `cluster.yaml.j2` sets the legacy `cluster.network.cni.name: none` *alongside* a multi-doc
  `KubeNetworkConfig` document. Tried the same here and `talosctl validate --strict` rejected it
  outright — `KubeNetworkConfig` (and separately `KubeFlannelCNIConfig`) both refuse to coexist
  with *any* populated legacy `cluster.network` block, confirmed via both `V1Alpha1ConflictValidate`
  in the Talos source and by reproducing the exact error against this cluster's own rendered
  config. This repo's `cluster.yaml` therefore has no `cluster.network` block at all — subnets come
  from the `KubeNetworkConfig` document alone, and CNI is disabled by omitting
  `KubeFlannelCNIConfig` entirely, per the point above. Why `home-ops`'s own config doesn't hit this
  is unconfirmed (possibly a Talos version difference, possibly `talosctl validate --strict` simply
  isn't part of his workflow) — not investigated further since this repo's own config now passes
  `talosctl validate --strict` cleanly on all 3 nodes, which is the standard that matters here.
- **ADR-0012 is effectively superseded.** Shipping zero admission-plugin configuration is now
  achieved by pure omission (no `KubeAdmissionControlConfig` document at all), which is what that
  ADR wanted to do before discovering `gen config`'s default forced an explicit delete. The
  underlying decision — no PodSecurity enforcement — is unchanged; only the mechanism is simpler.
- Version-tracked image references that `gen config --kubernetes-version`/`--install-image` used to
  compute automatically are now hand-maintained in the base document, interpolated from the same
  `versions.yaml` values via `envsubst`: `KubeAPIServerConfig`/`KubeControllerManagerConfig`/
  `KubeSchedulerConfig`/`KubeletConfig`'s `image` fields (`<registry path>:${KUBERNETES_VERSION}`,
  paths confirmed against `pkg/machinery/constants/constants.go`) and
  `UnattendedInstallConfig.installer.image` (`factory.talos.dev/installer/${TALOS_SCHEMATIC}:${TALOS_VERSION}`).
  Bumping `versions.yaml` continues to update all of them, just via string interpolation instead of
  a CLI flag doing the equivalent lookup.
- `KubeAPIServerConfig.certExtraSANs` and `machine.certSANs`, previously populated by
  `gen config --additional-sans`, are now explicit static lists (`127.0.0.1`, `10.0.10.245`) in the
  hand-authored base.
- `cluster.id`/`cluster.secret` are now explicitly populated from `talsecret.sops.yaml` —
  previously silently absent (Talos v1.14's `gen config` nils these legacy fields out whenever
  `DiscoveryIdentityMultidocConfig` is supported, i.e. any version past 1.13, and this repo had no
  multi-doc `DiscoveryIdentityConfig` to replace them), so cluster discovery identity had no
  stable, explicit value under the old pipeline.
- **Not adopted**: `DiscoveryServiceConfig`/`DiscoveryIdentityConfig` as multi-doc kinds —
  populating the legacy `cluster.id`/`cluster.secret` fields directly (above) reaches the same end
  state without adding two more documents.
- Same validation discipline as ADR-0013's own migration applies here: `just talos diff <hostname>`
  against all 3 nodes before any `apply-config`, watching specifically for anything the
  hand-authored base got wrong relative to what's actually live today (image tags, cert SANs, CNI
  state). Per this repo's convention, no agent runs `apply-config` against the live cluster — a
  human executes the cutover.
