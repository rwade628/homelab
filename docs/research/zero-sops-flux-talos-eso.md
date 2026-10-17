# Zero-sops patterns for Flux + Talos with ESO

Research for issue [#1638](https://github.com/rwade628/homelab/issues/1638), a child of the
map [#1634](https://github.com/rwade628/homelab/issues/1634). Researched 2026-10-17.

Method: this repo was read-only and only file paths, kustomizations and the non-secret manifests
that consume each Secret were read. No `sops -d` was run and no Secret data was read. External
claims cite primary sources: upstream docs/source pinned to a commit, and public home-ops repos
pinned to their HEAD on 2026-10-17. The [Sources](#sources) section lists every link.

## TL;DR

- **Three well-known Flux + Talos repos have no sops at all at HEAD:** onedr0p (removed
  2026-01-06, [`df998b64`][od-rm]), buroa (removed 2025-01/02, [`6123539c`][bu-rm1],
  [`582f4a6e`][bu-rm2]) and bjw-s (removed 2026-02-09, [`9d1d58f9`][bj-rm]). tholinka is a fourth,
  and the only one on **Bitwarden Secrets Manager**. All four use the same three-part shape:
  1. **Talos:** the machine config templates hold vault *references*, not values, and a CLI the
     human runs resolves them at render time (`minijinja-cli … | op inject` in
     onedr0p/buroa/bjw-s, `| vals flatten` in tholinka). The secrets bundle never touches git.
  2. **Bootstrap:** before Helm runs, `just bootstrap` renders a few plain `Secret` manifests through
     the same inject step and `kubectl apply`s them. The **only** credential that must exist is the
     ESO provider's root credential, plus at most one or two values that Flux needs before ESO is up.
  3. **Steady state:** an `ExternalSecret` for everything else. In three of the repos, that
     includes an `ExternalSecret` that *adopts* the root credential seeded at bootstrap, using ESO's
     documented "Owner" adoption of a Secret that has no ownerReference.
- **The irreducible bootstrap secret** is the human's interactive vault login on the workstation.
  Everything else hangs off it: one machine credential for ESO (a 1Password service-account token
  or Connect creds+token, a Bitwarden SM machine-account access token, or a Keeper KSM config JSON),
  plus the Talos secrets bundle. `talosconfig` is derived from the bundle.
- **`cluster-secrets` + `postBuild.substituteFrom` is the pitfall.** onedr0p and buroa removed the
  cluster-wide secret substitution rather than moving it to ESO. They hard-code the domain and,
  where something has to be substituted, use a per-app Secret seeded at bootstrap. This repo
  creates its Flux Kustomizations in *every app namespace*, and Flux resolves `substituteFrom` in
  the Kustomization's own namespace. The root `cluster-apps` also requires `cluster-secrets` with
  `optional: false`, so an ESO-generated `cluster-secrets` built *under* `cluster-apps` deadlocks.
  The low-risk move is to de-secret `SECRET_DOMAIN`/`TAILSCALE_DOMAIN` into a plain ConfigMap and to
  give the tunnel ID a per-app Secret.
- **Backend fit for the "no paid seat" constraint:** Keeper's own docs say *"Secrets Manager is
  available for Business accounts"*, so KSM is out on a Family plan. Bitwarden SM has a Free
  organization tier: 2 users, 3 projects, 3 machine accounts. A Families org can't add SM; Bitwarden
  says to create a separate Free org. The ESO provider needs the extra `bitwarden-sdk-server` behind
  a cert-manager TLS cert, and `bws` has **no template-inject command**. `bws run` injects env vars,
  which suits this repo's existing env-var-based `talos/mod.just`. 1Password Individual/Families
  service accounts work with ESO's `onepasswordSDK` provider but are capped at **1,000 requests/day
  per account**.
- **This repo's bootstrap is already broken in ways a zero-sops design has to fix anyway.**
  `bootstrap/mod.just` isn't registered in `.justfile` and calls `just template` and
  `just talos apply-node`, neither of which exists. `bootstrap/helmfile/apps.yaml` `needs:` a
  nonexistent `external-secrets/onepassword-connect` release, and there's no
  `kubernetes/apps/external-secrets/` for its values template to read.
  `scripts/bootstrap-apps.sh` and `.taskfiles/bootstrap/Taskfile.yaml` (talhelper, `kubernetes/bootstrap/…`,
  `kubernetes/flux/vars/…`) point at paths that no longer exist.

## This repo today (read-only findings)

- `.sops.yaml` has two rules with the same age recipient: `talos/*.sops.yaml` (whole file) and
  `(bootstrap|kubernetes)/*.sops.yaml` (`data|stringData` only).
- `talos/mod.just` `render` runs `secrets=$(sops -d talsecret.sops.yaml)`, exports about 14 env vars
  with `yq`, then runs `yq … envsubst` over the hand-authored `cluster.yaml`/`controlplane.yaml`
  (ADR-0014). The *only* sops coupling is that one line. Every other line is backend-neutral.
- `kubernetes/flux/cluster/ks.yaml` (`cluster-apps`, ns `flux-system`) sets
  `decryption.secretRef: sops-age` and `substituteFrom: cluster-secrets (optional: false)` plus
  `cluster-settings (optional: true)`. It also patches both onto every child Kustomization.
- **No `cluster-settings` ConfigMap exists in the repo.** It's referenced only in
  `kubernetes/flux/cluster/ks.yaml`, so in practice every substituted variable comes from
  `cluster-secrets`. The manifests consume `${SECRET_DOMAIN}` (46 uses), `${TAILSCALE_DOMAIN}`
  (3, all in `attic/.../server.toml`) and `${SECRET_CLOUDFLARE_TUNNEL_ID}` (1,
  `network/cloudflare-tunnel/app/dnsendpoint.yaml`). The other `${...}` variables (`APP`,
  `VOLSYNC_*`) come from inline `postBuild.substitute`.
- Each namespace `kustomization.yaml` (13 namespaces) includes `components/sops`. That copies
  `cluster-secrets`, `sops-age` and `ghcr-secret` into the namespace, because the Flux
  Kustomization objects themselves live there and Flux resolves `decryption.secretRef` and
  `substituteFrom` in the Kustomization's own namespace ([Flux API][flux-subref]: *"Should reside
  in the same namespace as the referring resource"*).
- The `flux-system` GitRepository is `https://github.com/rwade628/homelab.git` with no
  `secretRef` (`flux-instance` HelmRelease `sync`). This is a public repo, so **Flux needs no git
  credential to start**.
- Bootstrap staging (`bootstrap/mod.just`): `base` runs
  `kustomize build bootstrap/kustomize/apps | just template - | kubectl apply`, then CRDs. `apps`
  runs `helmfile sync`. This is the onedr0p layout, minus his `.justfile` `template` recipe
  (`minijinja-cli … | op inject`), so the `base` stage can't run here as written.

## Inventory: the 24 sops files and their replacements

There are **24 encrypted `*.sops.yaml` files** (`find . -name '*.sops.*'`), plus the `.sops.yaml`
config in row 1. The pattern letters
refer to [Patterns](#patterns). The **Consumer evidence** column comes from non-secret manifests
in the same directory. Where nothing in git names the Secret, the entry says so. A metadata-only
script the human runs can confirm those cases.

| # | File | Category | Consumer evidence (non-secret files) | Replace with |
|---|---|---|---|---|
| 1 | `.sops.yaml` | sops config | n/a | **Delete** once the last encrypted file is gone |
| 2 | `talos/talsecret.sops.yaml` | Talos machine secrets bundle | `talos/mod.just` `render` | **T**: vault item(s), resolved at render time |
| 3 | `kubernetes/components/sops/sops-age.sops.yaml` | sops decryption key (copied into every ns) | `decryption.secretRef` in `flux/cluster/ks.yaml` | **Delete** (nothing to decrypt). Remove `decryption:` from `cluster-apps` and its child patch, and drop the decryption flag from the `flux-instance` patches if one exists (onedr0p [`df998b64`][od-rm] diff) |
| 4 | `kubernetes/components/sops/cluster-secrets.sops.yaml` | Flux substitution vars | `${SECRET_DOMAIN}`, `${TAILSCALE_DOMAIN}`, `${SECRET_CLOUDFLARE_TUNNEL_ID}` | **S**: de-secret into a committed ConfigMap, or a per-app seeded Secret for the tunnel ID. Avoid an ESO-generated, cluster-wide `cluster-secrets` (see pitfalls) |
| 5 | `kubernetes/components/sops/ghcr-secret.sops.yaml` | Image pull secret (copied into all 13 ns) | `imagePullSecrets: [ghcr]` only in `yarr/webhook` and `yarr/channels` | **E** scoped to `yarr` only (`ExternalSecret` with a `kubernetes.io/dockerconfigjson` template), or a `ClusterExternalSecret` with a namespace selector. ESO's `GithubAccessToken` generator is an option if a GitHub App is acceptable |
| 6 | `bootstrap/kustomize/apps/network/secret.sops.yaml` | Pre-Flux seed into `network` | Not named in any non-secret file; layout matches onedr0p's `cloudflare-tunnel-id-secret` seed | **B** if a pre-ESO value is truly needed. Otherwise **E** and delete. Confirm the Secret's name/keys with a metadata-only script |
| 7 | `kubernetes/apps/flux-system/flux-instance/app/secret.sops.yaml` | Flux Receiver webhook token | `receiver.yaml` → `github-webhook-token-secret` | **E** (onedr0p does exactly this: [`flux-instance/app/externalsecret.yaml`][od-flux-es]) |
| 8 | `kubernetes/apps/flux-system/konflate/app/secret.sops.yaml` | Webhook token | `existingSecret: konflate-webhook-token-secret` | **E** |
| 9 | `kubernetes/apps/cert-manager/cert-manager/app/secret.sops.yaml` | Cloudflare DNS-01 API token | `clusterissuer.yaml` `apiTokenSecretRef: cert-manager-secret` | **E**. Not needed before ESO: ClusterIssuers arrive via Flux, after ESO |
| 10 | `kubernetes/apps/network/cloudflare-dns/app/secret.sops.yaml` | Cloudflare API token (external-dns) | `secretKeyRef: cloudflare-dns-secret` | **E** |
| 11 | `kubernetes/apps/network/cloudflare-tunnel/app/secret.sops.yaml` | Tunnel credentials | `envFrom: cloudflare-tunnel-secret` | **E** (onedr0p builds `TUNNEL_TOKEN` from vault fields in [`cloudflare-tunnel/app/externalsecret.yaml`][od-cft-es]) |
| 12 | `kubernetes/apps/o11y/grafana/app/secret.sops.yaml` | App credential (likely admin) | No Secret name in the HelmRelease. Check with the metadata script | **E**, or **G** + PushSecret if the value can be generated |
| 13 | `kubernetes/apps/o11y/kube-prometheus-stack/app/secret.sops.yaml` | Alertmanager receiver creds | `alertmanagerconfig.yaml` → `alertmanager-secret` | **E** |
| 14 | `kubernetes/apps/o11y/mktxp/app/secret.sops.yaml` | Router API creds | HelmRelease mounts a `type: secret` volume | **E** |
| 15 | `kubernetes/apps/o11y/omada-exporter/app/secret.sops.yaml` | Omada API creds | `envFrom: omada-exporter-credentials` | **E** |
| 16 | `kubernetes/apps/rook-ceph/rook-ceph/app/secret.sops.yaml` | Rook-related credential | No Secret name in the HelmRelease. Check with the metadata script | **E** |
| 17 | `kubernetes/apps/tailscale/operator/app/secret.sops.yaml` | Tailscale OAuth client | Chart default secret name (not referenced explicitly) | **E** |
| 18 | `kubernetes/apps/tailscale/route-pruner/app/secret.sops.yaml` | Tailscale API creds | `envFrom: route-pruner-secret` | **E** |
| 19 | `kubernetes/apps/home-assistant/mcp/app/secret.sops.yaml` | HA access token | `envFrom: {{ .Release.Name }}-secret` | **E** |
| 20 | `kubernetes/apps/attic/attic/app/secret.sops.yaml` | Attic server secret | `secretKeyRef: attic-secret` | **E**. A signing key whose rotation invalidates tokens, so the vault stays the source of truth |
| 21 | `kubernetes/apps/yarr/recyclarr/app/secret.sops.yaml` | *arr API keys | `envFrom: recyclarr-secret` | **E** |
| 22 | `kubernetes/apps/yarr/webhook/app/secret.sops.yaml` | Webhook/API keys | `envFrom: webhook-secret` | **E** |
| 23 | `kubernetes/apps/volsync-system/kopia/app/secret.sops.yaml` | Kopia repo creds | `envFrom: kopia-secret` | **E** |
| 24 | `kubernetes/apps/volsync-system/volsync/maintenance/secret.sops.yaml` | Kopia maintenance repo creds | `kopiamaintenance.yaml` `repository: volsync-maintenance-secret` | **E** |
| 25 | `kubernetes/components/volsync/secret.sops.yaml` | Per-app Kopia repo secret (component) | `replicationsource.yaml` `repository: ${APP}-volsync-secret` | **E** inside the component, with `${APP}` still substituted from the inline `postBuild.substitute` (onedr0p's [`components/kopiur/secret/externalsecret.yaml`][od-kopiur-es] is the analog) |

(Rows 2–25 are the 24 encrypted files. Row 6, the bootstrap seed, is easy to miss: it sits
outside `kubernetes/`.)

Net result for the 24 encrypted files: **20 become `ExternalSecret`s** (rows 5, 7–25), **1 goes to the render-time Talos pattern**,
**1 becomes a bootstrap seed or disappears**, **1 is de-secreted**, and **1 is deleted outright**
(`sops-age`). `.sops.yaml` goes too. Only the ESO root credential is new: it becomes a bootstrap seed and
lives in no file.

## Patterns

### T: Talos secrets resolved from the vault at render time

**What the reference repos do.** onedr0p's `talos/cluster.yaml.j2` and `controlplane.yaml.j2`
inline `op://kubernetes/talos/<FIELD>` for every bundle field: machine CA, cluster/aggregator/etcd
CAs, SA key, tokens, `DiscoveryIdentityConfig` id/secret, and the secretbox key
([`cluster.yaml.j2`][od-tal-cluster], [`controlplane.yaml.j2`][od-tal-cp]).
[`talos/mod.just`][od-tal-just] renders with `talosctl machineconfig patch <(just template
cluster.yaml.j2) -p @<(just template …)`. The `template` recipe in [`.justfile`][od-justfile] is
`minijinja-cli "{{ file }}" | op inject`. bjw-s uses the identical recipe
([`.justfile`][bj-justfile], [`kubernetes/talos/mod.just`][bj-tal-just], `op://infrastructure/talos/…`).
buroa does the same ([`.justfile`][bu-justfile]). tholinka keeps a `talos/secrets.yaml` whose
values are `ref+onepasswordconnect://home-ops/talos#-.<field>` and resolves them with
`minijinja-cli … | vals flatten` ([`talos/secrets.yaml`][th-tal-sec], [`.justfile`][th-justfile]).

**Upstream basis.** Talos's production notes: generate the bundle once with `talosctl gen secrets -o
secrets.yaml`, then `talosctl gen config --with-secrets secrets.yaml …` ([Talos prodnotes][tal-prod]).
The CLI reference also has `talosctl gen secrets --from-controlplane-config`, which *rebuilds a bundle
from a controlplane machine config* ([talosctl CLI ref][tal-cli]). That makes it the recovery path
if the vault copy is ever lost while a node still runs. The prodnotes page gives no storage advice.

**Mapping onto this repo.** `talos/mod.just` already substitutes from env vars, so there are two
low-diff options:

1. **Whole bundle as one vault item.** Replace `secrets=$(sops -d talsecret.sops.yaml)` with a
   single vault read of the stored bundle YAML and keep every `yq` line. This is backend-agnostic:
   `op read op://…`, `bws secret get <id>` piped through `jq -r .value`, or a KSM record field/file.
2. **One vault field per value.** Drop the `yq` extraction and inject env vars straight from the
   vault: `bws run --project-id <talos-project> -- …` (Bitwarden SM injects secrets *by name* as env
   vars: [bws CLI][bws-cli]), `ksm exec -- …` with `keeper://` values ([ksm exec][ksm-exec]), or
   `op run`. Alternatively, switch the templates to inline references and pipe through `op inject` /
   `ksm interpolate` (reads stdin, has `--validate`: [ksm interpolate][ksm-interp]) /
   `vals flatten`.

**Backend-specific facts.**

| Backend | Template inject | Env-var inject | Notes |
|---|---|---|---|
| 1Password | `op inject` (`op://` refs) | `op run` | The reference repos all use `op inject` |
| Bitwarden SM | **none** in `bws` | `bws run` (names → env vars, `--uuids-as-keynames`, `--project-id`) | vals' `bitwarden` backend is Password Manager via `bw serve`, **not** SM ([vals README][vals]). Use `bws run` or `bws secret get` + `jq` |
| Keeper KSM | `ksm interpolate` (stdin, `:-default`, `--validate`) | `ksm exec` | Requires KSM (Business-only, see below) |
| vals (multi) | `vals eval`/`flatten` with `ref+…` | `vals exec` | Backends include `onepassword`, `onepasswordconnect`, `bitwarden` (PM), `sops`, `exec`, … No Keeper and no BWS ([vals README][vals]) |

**What breaks when the vault is down at render time.**

- Rendering fails, so `apply-config`/`diff` and adding or rebuilding a node are blocked. Running
  nodes are unaffected: they hold their own config. This repo's `upgrade-node`, `reboot` and
  `upgrade-k8s` don't render, so they keep working, as do `talosctl` operations that use the
  existing local `talosconfig`.
- **Fail closed, or you push a broken config.** In bash, `set -e`/`pipefail` do **not** propagate a
  failure from inside `<( … )` process substitution, and onedr0p/bjw-s render with
  `talosctl machineconfig patch <(just template …)`. Keep this repo's current shape: command
  substitution into a variable, or temp files, both of which `set -e` does catch. Add a guard that
  the rendered output contains no unresolved `op://`/`keeper://`/`ref+` strings and no empty secret
  fields before `apply-config`. `ksm interpolate --validate` does the check for Keeper.
- Disaster fallback: as long as one control-plane node is up, `talosctl gen secrets
  --from-controlplane-config` can rebuild the bundle from its config ([talosctl CLI ref][tal-cli]).

### B: Bootstrap seed rendered by the human's vault CLI

**What the reference repos do.** onedr0p's `bootstrap/kustomize/personal/` contains *plain*
`Secret` manifests whose values are `op://` references:
[`external-secrets/secret.yaml`][od-boot-eso] holds the 1Password Connect credentials JSON and
token, and [`network/secret.yaml`][od-boot-net] holds `cloudflare-tunnel-id-secret`. The `base` stage
of [`bootstrap/mod.just`][od-boot-just] runs `kustomize build … | just template - | kubectl apply`
before Helm. His [`bootstrap/README.md`][od-boot-readme] states the intent: *"bootstrap Secrets
rendered through `op inject`… These exist before their controllers so nothing deadlocks on a missing
Secret."* buroa seeds a single Secret, the 1Password **service-account** token
([`bootstrap/kustomize/apps/external-secrets/secret.yaml`][bu-boot-eso]). bjw-s seeds only the
Connect creds+token ([`kubernetes/bootstrap/kustomize/external-secrets/secret.yaml`][bj-boot-eso]).
tholinka (Bitwarden SM) seeds the machine key, org ID and project ID via `vals`
([`bootstrap/resources.yaml.j2`][th-boot-res]).

**ClusterSecretStore during bootstrap.** onedr0p's helmfile applies `clustersecretstore.yaml` in a
`postsync` hook of the `onepassword-connect` release after waiting for the
`clustersecretstores.external-secrets.io` CRD ([`bootstrap/helmfile/apps.yaml`][od-helmfile]).
buroa applies a `stores/` kustomization from the `external-secrets` release's postsync hook and
pins `--field-manager=kustomize-controller`, so Flux owns the objects afterwards
([`bootstrap/helmfile/apps.yaml`][bu-helmfile]).

**Seed, then adopt.** All three of onedr0p, buroa and tholinka declare an `ExternalSecret` whose
*target is the seeded root Secret itself* ([od][od-1p-es], [buroa][bu-store], [tholinka][th-es]).
tholinka's comment reads: *"this secret gets manually created at bootstrap, and then this takes
control of it"*. ESO documents the mechanism: under the default `creationPolicy: Owner`, *"If the
secret exists and the ownerReference field is not found, the controller treats this secret as
orphaned. It will take ownership of this secret by adding an `ownerReference`"*
([ESO ownership/deletion policy][eso-own]).

**Self-reference risks to weigh.** Once adopted, the root credential carries an `ownerReference` to
its `ExternalSecret`. Pruning or deleting that `ExternalSecret` garbage-collects the credential the
store depends on, and the store can't refetch it. Rotation in the vault does propagate, but a bad
value locks the store out until it's re-seeded by hand. bjw-s does **not** adopt: his `onepassword-
connect` app has only the store ([tree at HEAD][bj-store]). The non-adopting alternative is to leave
the root Secret unmanaged, so it's created only by bootstrap and is invisible to Flux.

### E: Steady-state `ExternalSecret` per consumer

The default replacement for 20 of the 24 encrypted files. The pattern in every reference repo: a
`ClusterSecretStore`, then per-app `externalsecret.yaml` files using `dataFrom.extract` of one vault
item plus a `target.template` that renames or packs keys (e.g. onedr0p's tunnel token
[`cloudflare-tunnel/app/externalsecret.yaml`][od-cft-es]). For readiness gating, onedr0p puts
`healthChecks` and a CEL `healthCheckExprs` on the `ClusterSecretStore` `Ready` condition in the
store's Flux Kustomization ([`onepassword-connect/ks.yaml`][od-1p-ks]). buroa does the same in a
separate `external-secrets-stores` Kustomization ([`external-secrets/ks.yaml`][bu-eso-ks]).
Consumers don't `dependsOn` the store in either repo. They rely on ESO and the pods retrying.

**Provider specifics** (backend-specific, from ESO docs at [`9f7eb47f`][eso-tree]):

| | 1Password (`onepasswordSDK`) | 1Password Connect (`onepassword`) | Bitwarden SM (`bitwardensecretsmanager`) | Keeper (`keepersecurity`) |
|---|---|---|---|---|
| Root credential | Service-account token ([buroa store][bu-store]) | Connect creds JSON + token (2 secrets) | Machine-account access token ([ESO BW SM][eso-bw]) | KSM config, base64 JSON ([ESO Keeper][eso-keeper]) |
| Extra in-cluster parts | none | Connect server Deployment | `bitwarden-sdk-server` (chart dep) **and an HTTPS cert**: *"The Bitwarden SDK Server NEEDS to run as an HTTPS service"*. tholinka issues it from cert-manager ([`certificate.yaml`][th-cert]) | none |
| Store scope | per vault | per vault list | *"1 store == 1 organization/project"* | per KSM app/folder |
| Lookup | `<item>/[section/]<field>` | item/field | UUID or name. **`dataFrom.find` ignores filters and returns every secret, keyed by UUID** ([ESO BW SM][eso-bw]) | record UID + field label |
| PushSecret | yes | yes | yes, with restrictions | yes (needs `folderID`, write perms) |
| Plan constraint | Individual/Families: **1,000 requests/day per account** across all service accounts ([1P rate limits][op-rl]) | (plan eligibility not checked) | Free org: 2 users, 3 projects, 3 machine accounts. Families can't add SM, *"create a new Free organization"* ([BW SM plans][bw-plans]) | *"Secrets Manager is available for Business accounts"* ([KSM quick start][ksm-qs]) |

Implications for #1634's constraints: **Keeper KSM is ruled out** without a business seat. With
1Password Families, size ESO's `refreshInterval` against the 1,000/day account-wide cap. The 20 files become about
33 `ExternalSecret`s, because `components/volsync` is included by 14 app `ks.yaml`s. On the default
1h refresh that's about 790 reads a day before restarts and retries. That's close to the cap, so
use a longer `refreshInterval` or the SDK provider's `cache`.
With Bitwarden SM there's an extra Deployment, a cert-manager `Certificate`/issuer, and a
dependency ordering of cert-manager → ESO → store. tholinka gates the store Kustomization on both
the cert and the store ([`external-secrets/ks.yaml`][th-eso-ks]).

### S: Substitution variables (`cluster-secrets`) without sops

**What the reference repos did.** onedr0p removed `cluster-secrets.secret.sops.yaml` in 2024-10 and
flipped the Secret to `optional: true` in the same commit ([`a8880070`][od-op]). By HEAD there's no
cluster-wide substitution at all. The domain is literal (`external.turbo.ac` in
[`dnsendpoint.yaml`][od-cft-dns]), and the only `substituteFrom` left is a per-app Secret seeded at
bootstrap, `cloudflare-tunnel-id-secret`, in [`cloudflare-tunnel/ks.yaml`][od-cft-ks]. That Secret
exists only in the bootstrap seed: GitHub code search for its name returns just the bootstrap
file, the ks and the README. buroa deleted all `substituteFrom` (cluster-settings and
cluster-secrets) from `cluster/ks.yaml` in [`6123539c`][bu-rm1]. A code search at HEAD finds no
`substituteFrom` in the repo. tholinka uses `substituteFrom` for the BW org/project IDs, and only
on the external-secrets Kustomization ([`external-secrets/ks.yaml`][th-eso-ks]).

**Flux semantics that matter** ([kustomize-controller spec][flux-ks], [API types][flux-subref],
[CHANGELOG v1.9.0][flux-cl]):

- `substituteFrom` objects must be *"in the same namespace as the referring resource"*. With
  per-namespace Flux Kustomizations, as in this repo, the substitution Secret has to exist in **all
  13 namespaces plus `flux-system`**. An ESO-based replacement therefore needs a
  `ClusterExternalSecret` with a namespace selector, or an `ExternalSecret` in a component per
  namespace.
- `optional: false` (the default) *"failed reconciliation if the referenced object is missing"*.
  `cluster-apps` sets `optional: false` and is the Kustomization that would *create* any
  `ExternalSecret` for it, so using ESO naively gives a **bootstrap deadlock**. That's why the
  current script seeds it out-of-band and why onedr0p flipped it to `optional: true`.
- With `optional: true`, the default `substituteStrategy: WithVariables` *skips substitution
  entirely* when no variables resolve. Unsubstituted `${SECRET_DOMAIN}` strings would then be
  applied as literals. Since kustomize-controller **v1.9.0 (2026-06-17)**, `StrictPostBuildSubstitutions`
  is **on by default**: *"the controller fails the reconciliation when a variable without a default
  value is referenced in the manifests but is missing from the input vars"*. That fails closed only
  when substitution actually runs. This repo's Flux version wasn't checked (the live cluster wasn't
  queried).
- The Flux envsubst fork does **not** support `${var:?error}` ([envsubst README][flux-envsubst]),
  so you can't make a single variable mandatory.
- When the Secret changes, the Kustomization doesn't re-render until the next interval, unless the
  Secret carries `reconcile.fluxcd.io/watch: Enabled`. An ESO `target.template.metadata.labels`
  can add that label.
- Kustomizations can `dependsOn` a Kustomization that health-checks the `ExternalSecret`/store. The
  root `cluster-apps` can't depend on its own children, though.

**Options for this repo, from least to most moving parts:**

1. **De-secret.** `SECRET_DOMAIN` already shows up in public DNS and certificate-transparency logs
   for any publicly trusted cert, and it's **already in plaintext in this repo**
   (`casadewade-com-production-tls` in `network/envoy-gateway/app/envoy.yaml`,
   `omada-controller/app/helmrelease.yaml`, `CLAUDE.md`). Put it, and probably `TAILSCALE_DOMAIN`, in a committed
   `cluster-settings` ConfigMap. The plumbing exists: `cluster-apps` already references
   `cluster-settings` with `optional: true`, and its child patch is in place. Whether the tailnet
   name counts as sensitive is the user's call. The ConfigMap still has to exist in every
   namespace, so turn `components/sops` into a `components/settings` with the ConfigMap.
2. **Per-app seeded Secret** for anything that has to stay out of git and is needed at render
   time. That's `SECRET_CLOUDFLARE_TUNNEL_ID`, used once, which onedr0p handles exactly this way.
   Its Kustomization lists a single `substituteFrom` entry, the seed from pattern B.
3. **ESO-generated `cluster-secrets`** via `ClusterExternalSecret`. This needs `optional: true` on
   `cluster-apps` *and* the root `cluster-apps` must not consume the variables itself. It also
   needs a recent kustomize-controller so strict mode catches the missing vars, plus
   `reconcile.fluxcd.io/watch`. It works, but it's the option the reference repos moved away from.

### G: Generators and PushSecret for values the cluster creates

- **Generators** ([ESO generator guide][eso-gen], [Password][eso-pw]): *"Every invocation creates a
  new set of values"* and *"you can not share the same value produced by a generator across
  different `ExternalSecrets`"*. ESO's `ExternalSecret` reference warns that with
  `refreshPolicy: CreatedOnce`, *recreating* the ExternalSecret (e.g. a GitOps prune/re-apply)
  regenerates the value. **The documented safe form is `refreshPolicy: CreatedOnce` +
  `spec.target.immutable: true`**, with `creationPolicy: Orphan` to survive deletion
  ([ESO ExternalSecret][eso-es]). tholinka defines a shared `ClusterGenerator` `pw`
  ([`clustergenerator.yaml`][th-gen]).
- **The rebuild caveat.** A generated value lives only in etcd. After a full rebuild, an app whose
  PVC is restored by VolSync still holds the *old* credential. Use generators only for credentials
  that nothing outside the cluster persists, or push them back to the vault.
- **PushSecret** ([ESO PushSecret guide][eso-push]): `updatePolicy: Replace` (default) or
  `IfNotExists` (*"the provider becomes the source of truth"*). Nothing is deleted from the
  provider unless `deletionPolicy: Delete` is set. *"Rotate Secrets"* combines a Generator with a
  PushSecret. A real-world example is bjw-s pushing operator-created Kanidm admin passwords and
  OAuth2 client secrets into 1Password ([`kanidm-operator/instance/pushsecret.yaml`][bj-push]).
- **In this repo** nothing has to be generated. Candidates are app-internal admin passwords
  (grafana, rook dashboard if that's what rows 12/16 hold) and the two webhook tokens. The webhook
  tokens also have to be pasted into GitHub, so a vault-sourced **E** is simpler than **G** +
  PushSecret.

### Secrets needed before ESO runs

| Bootstrap stage | Needs a secret? | Source |
|---|---|---|
| Talos apply / `talosctl bootstrap` | Talos bundle | **T** (render time, human CLI) |
| Cilium, CoreDNS, Spegel | No: no Secret resources here (from the `bootstrap/helmfile/apps.yaml` releases; Cilium's `networking.yaml` holds BGP/LB pools) | n/a |
| cert-manager (Helm install) | No. Its DNS-01 token is used by `ClusterIssuer`s that Flux applies later | **E** |
| ESO + store | **Yes: the root credential** (and, for Bitwarden SM, a TLS cert from cert-manager) | **B** |
| flux-operator / flux-instance | No git credential: the repo is public over HTTPS. The Receiver token can come later | **E** |
| `cluster-apps` first reconcile | Today, yes: `cluster-secrets` (`optional: false`) and `sops-age` | **S** removes the need |

## What this means for the sequencing ticket (not a decision)

- Order that avoids deadlock, following the reference repos: build the vault items → add pattern T
  to `talos/mod.just` (one line in option 1) → repair `bootstrap/` (register the module, add a
  `template` recipe for the chosen CLI, add the ESO store to the helmfile) → land **S** before
  removing `cluster-secrets` → flip files to **E** a namespace at a time → delete `sops-age`,
  `components/sops` and `.sops.yaml` last.
- The open choice that this research doesn't settle: whether the ESO root credential is *adopted*
  by an `ExternalSecret` (onedr0p/buroa/tholinka) or left unmanaged after bootstrap (bjw-s).

## Sources

Repos are pinned to their HEAD on 2026-10-17.

**This repo** (read-only, HEAD `64f8bc18`): `.sops.yaml`, `talos/mod.just`, `kubernetes/components/sops/kustomization.yaml`,
`kubernetes/flux/cluster/ks.yaml`, `bootstrap/helmfile/apps.yaml`, `bootstrap/mod.just`,
`bootstrap/kustomize/apps/**/kustomization.yaml`, `scripts/bootstrap-apps.sh`, `.justfile`, the
non-secret manifests next to each `secret.sops.yaml`.

**onedr0p/home-ops** @ `558313864a2fd80a3242605fd16c6276b7c60514`
- [`.justfile`][od-justfile], [`bootstrap/mod.just`][od-boot-just], [`bootstrap/README.md`][od-boot-readme], [`bootstrap/helmfile/apps.yaml`][od-helmfile]
- [`bootstrap/kustomize/personal/external-secrets/secret.yaml`][od-boot-eso], [`bootstrap/kustomize/personal/network/secret.yaml`][od-boot-net]
- [`talos/mod.just`][od-tal-just], [`talos/cluster.yaml.j2`][od-tal-cluster], [`talos/controlplane.yaml.j2`][od-tal-cp]
- [`onepassword-connect/app/externalsecret.yaml`][od-1p-es], [`onepassword-connect/ks.yaml`][od-1p-ks], [`flux-instance/app/externalsecret.yaml`][od-flux-es]
- [`cloudflare-tunnel/ks.yaml`][od-cft-ks], [`cloudflare-tunnel/app/dnsendpoint.yaml`][od-cft-dns], [`cloudflare-tunnel/app/externalsecret.yaml`][od-cft-es], [`components/kopiur/secret/externalsecret.yaml`][od-kopiur-es]
- Commits: [`a8880070` "feat(bootstrap): replace sops with op-cli" (2024-10-12)][od-op], [`df998b64` "chore: remove sops (#10312)" (2026-01-06)][od-rm]

**buroa/home-ops** (formerly `buroa/k8s-gitops`) @ `9908e43f41659247dc66a714fbc92ed247928595`
- [`.justfile`][bu-justfile], [`bootstrap/helmfile/apps.yaml`][bu-helmfile], [`bootstrap/kustomize/apps/external-secrets/secret.yaml`][bu-boot-eso]
- [`external-secrets/external-secrets/stores/onepassword.yaml`][bu-store], [`external-secrets/external-secrets/ks.yaml`][bu-eso-ks]
- Commits: [`6123539c` "feat(flux): goodbye sops decryption" (2025-01-16)][bu-rm1], [`582f4a6e` "chore(bootstrap): bye bye sops" (2025-02-16)][bu-rm2]

**bjw-s-labs/home-ops** (formerly `bjw-s/home-ops`) @ `7d8e178168040b62a5bcc415cb9eb8c5f7b663af`
- [`.justfile`][bj-justfile], [`kubernetes/talos/mod.just`][bj-tal-just], [`kubernetes/bootstrap/kustomize/external-secrets/secret.yaml`][bj-boot-eso]
- [`onepassword-connect/app/clustersecretstore.yaml`][bj-store], [`kanidm-operator/instance/pushsecret.yaml`][bj-push]
- Commit: [`9d1d58f9` "fix(flux): Remove sops" (2026-02-09)][bj-rm]

**tholinka/home-ops** (Bitwarden SM) @ `171d6045c6ce673a158252183d562bbf7f0406d9`
- [`.justfile`][th-justfile], [`bootstrap/resources.yaml.j2`][th-boot-res], [`talos/secrets.yaml`][th-tal-sec]
- [`external-secrets/ks.yaml`][th-eso-ks], [`app/clustersecretstore.yaml`][th-store], [`app/externalsecret.yaml`][th-es], [`app/certificate.yaml`][th-cert], [`app/clustergenerator.yaml`][th-gen]

**External Secrets Operator** docs @ [`9f7eb47f`][eso-tree]
- [Bitwarden Secrets Manager provider][eso-bw], [Keeper Security provider][eso-keeper], [1Password SDK provider][eso-1p]
- [Ownership & deletion policy][eso-own], [ExternalSecret (refreshPolicy)][eso-es], [Generators guide][eso-gen], [Password generator][eso-pw], [PushSecret guide][eso-push]

**Flux** (kustomize-controller @ `431bce8b`)
- [Kustomization spec (dependsOn, postBuild, watch label)][flux-ks], [`SubstituteReference` API type][flux-subref], [CHANGELOG v1.9.0][flux-cl], [envsubst README][flux-envsubst]

**Talos**
- [Production Clusters notes (v1.14)][tal-prod], [talosctl CLI reference (v1.15, siderolabs/talos @ `d21101a9`)][tal-cli]

**Vault CLIs and plans**
- [vals README @ `333921f6`][vals], [Bitwarden `bws` CLI][bws-cli], [Bitwarden SM plans][bw-plans]
- [Keeper `ksm exec`][ksm-exec], [Keeper `ksm interpolate`][ksm-interp], [Keeper SM quick start][ksm-qs], [1Password service-account rate limits][op-rl]

[od-justfile]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/.justfile
[od-boot-just]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/bootstrap/mod.just
[od-boot-readme]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/bootstrap/README.md
[od-helmfile]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/bootstrap/helmfile/apps.yaml
[od-boot-eso]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/bootstrap/kustomize/personal/external-secrets/secret.yaml
[od-boot-net]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/bootstrap/kustomize/personal/network/secret.yaml
[od-tal-just]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/talos/mod.just
[od-tal-cluster]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/talos/cluster.yaml.j2
[od-tal-cp]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/talos/controlplane.yaml.j2
[od-1p-es]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/apps/external-secrets/onepassword-connect/app/externalsecret.yaml
[od-1p-ks]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/apps/external-secrets/onepassword-connect/ks.yaml
[od-flux-es]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/apps/flux-system/flux-instance/app/externalsecret.yaml
[od-cft-ks]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/apps/network/cloudflare-tunnel/ks.yaml
[od-cft-dns]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/apps/network/cloudflare-tunnel/app/dnsendpoint.yaml
[od-cft-es]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/apps/network/cloudflare-tunnel/app/externalsecret.yaml
[od-kopiur-es]: https://github.com/onedr0p/home-ops/blob/558313864a2fd80a3242605fd16c6276b7c60514/kubernetes/components/kopiur/secret/externalsecret.yaml
[od-op]: https://github.com/onedr0p/home-ops/commit/a8880070316526428580c8aabdb3831680e0aa4e
[od-rm]: https://github.com/onedr0p/home-ops/commit/df998b64f18c8dd2d396289ce8aae95013e84de2
[bu-justfile]: https://github.com/buroa/home-ops/blob/9908e43f41659247dc66a714fbc92ed247928595/.justfile
[bu-helmfile]: https://github.com/buroa/home-ops/blob/9908e43f41659247dc66a714fbc92ed247928595/bootstrap/helmfile/apps.yaml
[bu-boot-eso]: https://github.com/buroa/home-ops/blob/9908e43f41659247dc66a714fbc92ed247928595/bootstrap/kustomize/apps/external-secrets/secret.yaml
[bu-store]: https://github.com/buroa/home-ops/blob/9908e43f41659247dc66a714fbc92ed247928595/kubernetes/apps/external-secrets/external-secrets/stores/onepassword.yaml
[bu-eso-ks]: https://github.com/buroa/home-ops/blob/9908e43f41659247dc66a714fbc92ed247928595/kubernetes/apps/external-secrets/external-secrets/ks.yaml
[bu-rm1]: https://github.com/buroa/home-ops/commit/6123539c8f8978bbb0811c49875525c523c4360e
[bu-rm2]: https://github.com/buroa/home-ops/commit/582f4a6e2e01e1cb8fa68e8f0ce824a7ff3820d0
[bj-justfile]: https://github.com/bjw-s-labs/home-ops/blob/7d8e178168040b62a5bcc415cb9eb8c5f7b663af/.justfile
[bj-tal-just]: https://github.com/bjw-s-labs/home-ops/blob/7d8e178168040b62a5bcc415cb9eb8c5f7b663af/kubernetes/talos/mod.just
[bj-boot-eso]: https://github.com/bjw-s-labs/home-ops/blob/7d8e178168040b62a5bcc415cb9eb8c5f7b663af/kubernetes/bootstrap/kustomize/external-secrets/secret.yaml
[bj-store]: https://github.com/bjw-s-labs/home-ops/blob/7d8e178168040b62a5bcc415cb9eb8c5f7b663af/kubernetes/apps/external-secrets/onepassword-connect/app/clustersecretstore.yaml
[bj-push]: https://github.com/bjw-s-labs/home-ops/blob/7d8e178168040b62a5bcc415cb9eb8c5f7b663af/kubernetes/apps/security/kanidm-operator/instance/pushsecret.yaml
[bj-rm]: https://github.com/bjw-s-labs/home-ops/commit/9d1d58f9f0832fc8048e85ca50e65060a478f7db
[th-justfile]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/.justfile
[th-boot-res]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/bootstrap/resources.yaml.j2
[th-tal-sec]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/talos/secrets.yaml
[th-eso-ks]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/kubernetes/apps/security/external-secrets/ks.yaml
[th-store]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/kubernetes/apps/security/external-secrets/app/clustersecretstore.yaml
[th-es]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/kubernetes/apps/security/external-secrets/app/externalsecret.yaml
[th-cert]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/kubernetes/apps/security/external-secrets/app/certificate.yaml
[th-gen]: https://github.com/tholinka/home-ops/blob/171d6045c6ce673a158252183d562bbf7f0406d9/kubernetes/apps/security/external-secrets/app/clustergenerator.yaml
[eso-tree]: https://github.com/external-secrets/external-secrets/tree/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs
[eso-bw]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/provider/bitwarden-secrets-manager.md
[eso-keeper]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/provider/keeper-security.md
[eso-1p]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/provider/1password-sdk.md
[eso-own]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/guides/ownership-deletion-policy.md
[eso-es]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/api/externalsecret.md
[eso-gen]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/guides/generator.md
[eso-pw]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/api/generator/password.md
[eso-push]: https://github.com/external-secrets/external-secrets/blob/9f7eb47f2d28914b4fa70b342d88fcb3aab17cd9/docs/guides/pushsecrets.md
[flux-ks]: https://github.com/fluxcd/kustomize-controller/blob/431bce8b32ee5e05ece4f7c4973028aca84e660c/docs/spec/v1/kustomizations.md
[flux-subref]: https://github.com/fluxcd/kustomize-controller/blob/431bce8b32ee5e05ece4f7c4973028aca84e660c/api/v1/kustomization_types.go
[flux-cl]: https://github.com/fluxcd/kustomize-controller/blob/431bce8b32ee5e05ece4f7c4973028aca84e660c/CHANGELOG.md
[flux-envsubst]: https://github.com/fluxcd/pkg/blob/main/envsubst/README.md
[tal-prod]: https://docs.siderolabs.com/talos/v1.14/getting-started/prodnotes
[tal-cli]: https://github.com/siderolabs/talos/blob/d21101a9970b41f3beeffe5a7f6957351192f233/website/content/v1.15/reference/cli.md
[vals]: https://github.com/helmfile/vals/blob/333921f609792d6f564044bf621cc4f61b70d388/README.md
[bws-cli]: https://bitwarden.com/help/secrets-manager-cli/
[bw-plans]: https://bitwarden.com/help/secrets-manager-plans/
[ksm-exec]: https://docs.keeper.io/en/keeperpam/secrets-manager/secrets-manager-command-line-interface/exec-command
[ksm-interp]: https://docs.keeper.io/keeperpam/secrets-manager/secrets-manager-command-line-interface/interpolate-command
[ksm-qs]: https://docs.keeper.io/keeperpam/secrets-manager/quick-start-guide
[op-rl]: https://www.1password.dev/service-accounts/rate-limits/
