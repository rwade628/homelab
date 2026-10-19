# Secret delivery after sops: Bitwarden SM through one ClusterSecretStore, two seeds, Talos at render time

Sops goes away completely. Every in-cluster secret becomes an `ExternalSecret` that reads the
**Machine-Secrets Backend** (Bitwarden Secrets Manager, in its own Free org) through one
`ClusterSecretStore`. Talos's secrets bundle is fetched at render time by the human's `bws` CLI.
Bootstrap seeds exactly two Secrets that live in no file. Decided in
[#1642](https://github.com/rwade628/homelab/issues/1642), under the map
[#1634](https://github.com/rwade628/homelab/issues/1634). The patterns follow onedr0p, buroa,
bjw-s and tholinka; see the `research/zero-sops-flux-talos-eso` branch.

## Decision

**Backend layout.** The Free tier caps us at 3 projects and 3 machine accounts, and a store
can point at only one project. That rules out one project per app or per namespace. There are
two projects:

- `homelab` holds every secret the cluster reads.
- `bootstrap` holds only what the operator needs: the Talos bundle, the `eso` access token and
  the tailnet name.

There are two machine accounts:

- `eso` has *Can read* on `homelab`. The ESO docs ask for read-write, but nothing here uses
  PushSecret, so read-only comes first. Upgrade it only if the store won't go Ready.
- `operator` has read-write on both projects and is used by the `just` recipes. `bws` only
  accepts machine-account tokens, so there's no way to log in as a human. The `operator` token
  lives in the Human Vault's Homelab ▸ Break-glass folder. Recipes take it from
  `BWS_ACCESS_TOKEN` or prompt for it, and never write it to disk.

The third project and the third machine account stay spare.

**Secret shape.** There's one BWS secret per **Credential Source**: `cloudflare`, `tailscale`,
`kopia`, `omada` and so on, rather than one per consuming app. Names are lower-kebab and unique
across the whole org, because `bws run` aborts on a duplicate and the provider errors on one.
Each value is a JSON map with UPPER_SNAKE keys. Consumers use `dataFrom.extract` plus
`target.template`. The provider ignores `remoteRef.property`. Consumers reference secrets by
name, not UUID, so the manifests stay readable. Rotating a source refreshes only the
ExternalSecrets that read it, and every consumer carries the Reloader annotation.

**Scoping.** There's one `ClusterSecretStore` named `bitwarden`, pointed at `homelab`. Its
`caProvider` is `bitwarden-tls-certs`/`ca.crt` in `external-secrets`. It has no namespace
conditions and there are no per-namespace `SecretStore`s. With one project and one reader
account, those would add objects without adding any isolation. Consumers don't `dependsOn` the
store. They rely on ESO and the pods retrying.

**ESO is a Flux app.** It's new in `kubernetes/apps/external-secrets/`; until now it existed
only as a helmfile release whose values file was missing. It holds:

- the chart, with `bitwarden-sdk-server.enabled: true`;
- the cert-manager self-signed CA chain that issues `bitwarden-tls-certs`, because the SDK server
  only serves HTTPS;
- a separate `stores/` Kustomization that health-checks the store's `Ready` condition.

The bootstrap helmfile reads the same values through `templates/*.gotmpl`.

**Substitution.** `cluster-secrets` is deleted rather than rebuilt through ESO, because an
ESO-generated cluster-wide Secret deadlocks `cluster-apps`. `CLUSTER_DOMAIN` and
`CLUSTER_CLOUDFLARE_TUNNEL_ID` (renamed from `SECRET_*`) go in the committed `cluster-settings`
ConfigMap, distributed by `components/settings`, which replaces `components/sops`. Neither value
is a credential. The domain is already in public DNS, CT logs and this repo, and the tunnel ID
can't do anything without the tunnel token. The tailnet name is kept out of git by choice: it's
a seeded Secret in `attic` only, behind a `substituteFrom` (`optional: false`) on the attic
Kustomization alone.

**Bootstrap.**

- The template leftover `needs: external-secrets/onepassword-connect` is removed, and
  `flux-operator` now needs `external-secrets/external-secrets`.
- The ESO release's presync hook applies the TLS chain.
- Its postsync hook waits for the CRDs, then applies the seeds and `stores/` with
  `--field-manager=kustomize-controller` so Flux takes them over cleanly.
- The seed recipe, run with the `operator` token, writes exactly two Secrets: the `eso` root
  credential and the attic tailnet Secret.
- The root credential stays **unmanaged** for the life of the cluster, like bjw-s and unlike
  onedr0p, buroa and tholinka. If an `ExternalSecret` adopted it, pruning that ExternalSecret
  would garbage-collect the credential the store itself needs. To rotate it, re-seed.
- The old `bootstrap/kustomize/apps/network` seed is deleted once a metadata-only check confirms
  what it is.

**Talos.** The `bootstrap` project stores the whole `talsecret` bundle as one secret, a few KB
against a 25,000-character limit. In `talos/mod.just`, `render` replaces `sops -d` with
`bws secret get <uuid> | jq -er .value`. The UUID is committed; it isn't sensitive. Every `yq`
line stays. Render keeps its command-substitution shape so `set -e` catches a failed fetch, and
it fails on any empty or `null` value before it outputs anything.

**Leftovers.** `ghcr-secret` becomes an ExternalSecret in `yarr` only, the sole consumer, and is
no longer copied into all 13 namespaces. `sops-age`, the `decryption:` blocks and `.sops.yaml`
are deleted last.

## Considered options

- **Seeding the substitution values into every namespace**, or fanning them out with a
  ClusterExternalSecret. Both were rejected. The fan-out only exists because substitution is
  patched onto every Kustomization, and the values hidden that way were already public.
- **Adopting the root credential with an ExternalSecret.** Rejected because of the
  self-referential garbage-collection risk described above.
- **One BWS secret per Talos field, injected with `bws run`.** Rejected because it changes 14
  secrets and the render script, for no gain over one bundle.

## Addendum (2026-10-19): one seed, not two

Planning the cutover ([#1649](https://github.com/rwade628/homelab/issues/1649)) found that the
`cluster-apps` patch in `kubernetes/flux/cluster/ks.yaml` **replaces** each child
Kustomization's `postBuild.substituteFrom` list rather than merging into it. The live
`attic`, `cloudflare-tunnel` and `reloader` Kustomizations carry exactly the patch's two
entries, whatever their own `ks.yaml` says. An attic-only `substituteFrom` would be silently
dropped.

So the tailnet name doesn't use Flux substitution. Its only consumer is attic's `server.toml`,
which becomes an ESO template: the committed ConfigMap holds `{{ .TAILNET }}`, and the attic
`ExternalSecret` renders `server.toml` into `attic-secret`, reading `TAILNET` from the
`tailscale` Credential Source. It still stays out of git. Nothing substitutes from that Secret,
so the deadlock that ruled out an ESO-built `cluster-secrets` doesn't apply.

Bootstrap now seeds exactly **one** Secret, the `eso` root credential. The `bootstrap` project
holds the Talos bundle and the `eso` token, and no longer the tailnet name.

Rejected: an extra `optional: true` entry in the global patch (a missing seed would render
`attic.` with no error), and a label-selected second patch variant (two copies of a
load-bearing patch to keep in sync).
