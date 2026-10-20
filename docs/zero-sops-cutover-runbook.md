# Zero-sops cutover runbook

Takes the cluster from 24 sops files to zero, following
[ADR-0016](./adr/0016-secret-delivery-bitwarden-sm-eso.md) and its one-seed addendum.
Decided in [#1649](https://github.com/rwade628/homelab/issues/1649), under the map
[#1634](https://github.com/rwade628/homelab/issues/1634). Vocabulary (**Credential Source**,
**Bootstrap Seed**, **Machine-Secrets Backend**) is in [CONTEXT.md](../CONTEXT.md).

## Ground rules

- **Commit, push, Flux.** No `kubectl apply`, `apply-ks` or `flux build | kubectl apply`. The
  only human-run commands that touch the cluster are `just bootstrap seed` and `just talos …`.
- **Agents never read secret values.** No `sops -d`, `bws`, or Secret `.data` values. The
  per-namespace gate reads Secret **key names** only (`jq '.data | keys'`). Every script handed
  to the human prints metadata only and allowlists what it prints.
- **Every flip can be undone until P4.** sops files, `sops-age` and the `decryption:` blocks all
  stay until P4. After P4b, rollback means the offline `age.key` copy plus re-adding files.

## Phases at a glance

| Phase | What changes | Gate |
|---|---|---|
| Prep | BWS org, projects, machine accounts, every secret populated. No repo change. | Names-only listing matches the table below |
| P0 | Talos `render` reads `bws` instead of `sops -d` | `render-check`: every node `identical` |
| P1 | ESO as a Flux app, store, canary; bootstrap repair; one seed | Store Ready, canary synced, static bootstrap dry run |
| P2 | `cluster-settings` ConfigMap, then `SECRET_*` → `CLUSTER_*` | `flux build` byte-identical before and after |
| P3 | Namespaces flip to ExternalSecrets, leaves first | Per-namespace gate |
| P4 | `cluster-secrets`, `decryption:`, `sops-age`, all sops tooling deleted | Zero `*.sops.*`, Flux all Ready |
| +30 d | `age.key` destroyed everywhere | — |

Soak time: leaf namespaces can flip one after another. Leave **24 h** after each of
`network`, `cert-manager` and `flux-system`, and **7 days** between the end of P3 and P4.

## Credential Sources

Settled from the metadata-only facts in `scripts/sops-metadata-check.sh`. Results:
- equal: the two Cloudflare API tokens, the network seed and tunnel token, the three Kopia
  passwords and repositories, and the radarr and sonarr keys;
- different: the two Tailscale OAuth clients, the Flux and konflate webhook tokens;
- the volsync component's repository is static, with no `${APP}`.

**`homelab` project** (read by the `eso` machine account through `ClusterSecretStore/bitwarden`):

| BWS secret | Keys |
|---|---|
| `cloudflare` | `DNS_API_TOKEN`, `TUNNEL_TOKEN` |
| `tailscale` | `OPERATOR_CLIENT_ID`, `OPERATOR_CLIENT_SECRET`, `ROUTE_PRUNER_CLIENT_ID`, `ROUTE_PRUNER_CLIENT_SECRET`, `TAILNET` |
| `kopia` | `PASSWORD`, `REPOSITORY` |
| `github` | `FLUX_WEBHOOK_TOKEN`, `KONFLATE_WEBHOOK_SECRET`, `KONFLATE_APP_CLIENT_ID`, `KONFLATE_APP_PRIVATE_KEY` |
| `home-assistant` | `MCP_TOKEN` |
| `pushover` | `ALERTMANAGER_TOKEN`, `USER_KEY` |
| `mikrotik` | `USERNAME`, `PASSWORD` |
| `omada` | `USER`, `PASS`, `CLIENT_ID`, `SECRET_ID` |
| `radarr` | `API_KEY` |
| `sonarr` | `API_KEY` |
| `attic` | `SERVER_TOKEN_HS256_SECRET_BASE64` |
| `grafana` | `ADMIN_USER`, `ADMIN_PASSWORD` |
| `rook-ceph` | `DASHBOARD_PASSWORD` |
| `ghcr` | `USERNAME`, `TOKEN` |
| `canary` | `OK` (value `1`; removed in P4c) |

**`bootstrap` project** (`operator` machine account only): `talsecret` (the whole decrypted
`talsecret.sops.yaml` as YAML text) and `eso-token` (the `eso` machine account's access token).

Humans also use `grafana`, `rook-ceph`, `omada`, `mikrotik` and `ghcr`. For those, BWS is
canonical and Keeper keeps the autofill copy, per ADR-0016.

## Target Secrets

Every target keeps its **current name and key names**, so consumers don't change. The
`target.template` maps BWS keys onto the old key names. There are two exceptions, mktxp and
attic, described in P3.

| Namespace | Target Secret | Keys ← `bws-secret.KEY` |
|---|---|---|
| home-assistant | `home-assistant-mcp-secret` | `HOMEASSISTANT_TOKEN` ← `home-assistant.MCP_TOKEN` |
| o11y | `grafana-admin-password-secret` | `GF_SECURITY_ADMIN_USER`, `GF_SECURITY_ADMIN_PASSWORD` ← `grafana.ADMIN_*` |
| o11y | `alertmanager-secret` | `ALERTMANAGER_PUSHOVER_TOKEN` ← `pushover.ALERTMANAGER_TOKEN`, `PUSHOVER_USER_KEY` ← `pushover.USER_KEY` |
| o11y | `omada-exporter-credentials` | `OMADA_USER`, `OMADA_PASS`, `OMADA_CLIENT_ID`, `OMADA_SECRET_ID` ← `omada.*` |
| o11y | `mktxp-secret` (**new name**) | `credentials.yaml` ← `mikrotik.USERNAME`/`PASSWORD` |
| yarr | `recyclarr-secret` | `RADARR_API_KEY`, `SONARR_API_KEY` ← `radarr.API_KEY`, `sonarr.API_KEY` |
| yarr | `webhook-secret` | `RADARR__AUTH__APIKEY`, `SONARR__AUTH__APIKEY` ← same |
| yarr | `ghcr` (`dockerconfigjson`) | `.dockerconfigjson` ← `ghcr.USERNAME`/`TOKEN`, `auth` built with `b64enc` |
| rook-ceph | `rook-ceph-dashboard-password` | `password` ← `rook-ceph.DASHBOARD_PASSWORD` |
| tailscale | `operator-oauth` | `client_id`, `client_secret` ← `tailscale.OPERATOR_*` |
| tailscale | `route-pruner-secret` | `TS_OAUTH_CLIENT_ID`, `TS_OAUTH_CLIENT_SECRET` ← `tailscale.ROUTE_PRUNER_*` |
| volsync-system | `kopia-secret` | `KOPIA_PASSWORD` ← `kopia.PASSWORD` |
| volsync-system | `volsync-maintenance-secret` | `KOPIA_PASSWORD`, `KOPIA_REPOSITORY` ← `kopia.*` |
| per app (component) | `${APP}-volsync-secret` | `KOPIA_PASSWORD`, `KOPIA_REPOSITORY` ← `kopia.*` |
| attic | `attic-secret` | `ATTIC_SERVER_TOKEN_HS256_SECRET_BASE64` ← `attic.*`, **plus** `server.toml` (template, `TAILNET` ← `tailscale.TAILNET`) |
| network | `cloudflare-dns-secret` | `api-token` ← `cloudflare.DNS_API_TOKEN` |
| network | `cloudflare-tunnel-secret` | `TUNNEL_TOKEN` ← `cloudflare.TUNNEL_TOKEN` |
| cert-manager | `cert-manager-secret` | `api-token` ← `cloudflare.DNS_API_TOKEN` |
| flux-system | `github-webhook-token-secret` | `token` ← `github.FLUX_WEBHOOK_TOKEN` |
| flux-system | `konflate-webhook-token-secret` | `KONFLATE_*` ← `github.KONFLATE_*` |

Dropped, not migrated: `SECRET_ACME_EMAIL` (no references), the `bootstrap/kustomize/apps/network`
seed (a duplicate of `cloudflare-tunnel-secret`), `cluster-secrets`, `sops-age`.

## Prep: populate the backend

Done by hand in the Bitwarden web app. A `/wizard` script for these steps is generated when the
work starts; see the [appendix](#appendix-backend-checklist). Values are copied from a local
`sops -d` into the web UI, never piped through `bws secret create`, which would put them in argv.

**Gate:** with the `operator` token, `bws secret list | jq -r '.[] | .key'` prints exactly
the names above, in both projects. That command prints names only; never run a bare
`bws secret list`, because it prints values. Optionally, extend `sops-metadata-check.sh` with an
"equal to BWS?" mode that prints one boolean per target key.

## P0: Talos renders from BWS

**P0a (one commit).**
- In `talos/mod.just`, `render` reads `secrets=$(bws secret get <talsecret-uuid> | jq -er .value)`.
  The UUID is committed. Every `yq` line stays. The fetch keeps its command-substitution shape
  so `set -e` catches it failing, and render fails on any empty or `null` field before it
  outputs anything.
- `render` takes its source from a private `secrets_source` variable that defaults to `bws`.
- Add a temporary `render-check` recipe. For every node it renders once with `sops` and once
  with `bws` into a throwaway `mktemp -d`. It prints `<host>: identical` or `<host>: DIFFERS`
  using `cmp -s`, never a diff body (the configs hold CA keys), and removes the directory on
  exit.

**Gate:** you run `just talos render-check`, and every node must print `identical`.

**Rollback:** revert P0a. `talsecret.sops.yaml` stays until P4c.

## P1: ESO, store, bootstrap

**P1a: ESO as a Flux app.** In `kubernetes/apps/external-secrets/`, with `components/alerts`:
- the `external-secrets` HelmRelease with `bitwarden-sdk-server.enabled: true`;
- the cert-manager self-signed CA chain that issues `bitwarden-tls-certs`;
- a separate `stores/` Kustomization with `ClusterSecretStore/bitwarden` (pointing at the
  `homelab` project, `caProvider` = `bitwarden-tls-certs`/`ca.crt`, auth from the seeded
  `eso` token), a health check on the store's `Ready` condition, and an `eso-canary`
  ExternalSecret reading `canary`.

Until the seed exists, `stores/` stays not-Ready. That's harmless, because nothing consumes it
yet.

**P1b: bootstrap repair.**
- Add `mod bootstrap` to `.justfile`.
- Remove the `just template` call from `base`. `base` now applies only namespaces and CRDs.
- Add a `seed` recipe. It reads `BWS_ACCESS_TOKEN` (the `operator` token) or prompts for it,
  never writes it to disk, and writes exactly one Secret: the `eso` root credential in
  `external-secrets`. Apply it with `--field-manager=kustomize-controller`, unmanaged by any
  ExternalSecret. `just bootstrap cluster` prompts for the token once and exports it, so
  helmfile hooks, which can't prompt, inherit it.
- In `bootstrap/helmfile/apps.yaml`:
  - `flux-operator` needs `external-secrets/external-secrets`, replacing the leftover
    `onepassword-connect`.
  - The ESO release gets its values from the new Flux app through `templates/values.yaml.gotmpl`.
  - The presync hook applies the TLS chain.
  - The postsync hook waits for the CRDs, runs `just bootstrap seed`, then applies `stores/`
    with `--field-manager=kustomize-controller`.
- Delete `bootstrap/kustomize/apps/network`, `scripts/bootstrap-apps.sh` and
  `.taskfiles/bootstrap`, along with the `bootstrap:` include in `Taskfile.yaml`. All of these
  are stale today.

Then you run `just bootstrap seed` once against the live cluster.

**Gate (store):**
1. `flux get ks -n external-secrets` shows `stores` Ready.
2. `kubectl get clustersecretstore bitwarden` shows `Ready=True`, capabilities `ReadOnly`.
   If it won't go Ready on read-only, give `eso` read-write on `homelab` (ADR-0016).
3. `kubectl get externalsecret -n external-secrets eso-canary` shows `SecretSynced`.

**Gate (bootstrap, static):**
- `helmfile -f bootstrap/helmfile/apps.yaml lint` and `template` pass;
- `just --dry-run bootstrap cluster` resolves every recipe;
- the TLS chain and `stores/` render and pass kubeconform.

Optional, not a gate: a rehearsal on a throwaway `talosctl cluster create` Docker cluster on
loki. The real proof is the next rebuild.

**Rollback:** revert P1a and P1b. Nothing reads the store yet.

## P2: settings before secrets

`substituteFrom` replaces a missing `${VAR}` with an empty string, and `cluster-settings` is
`optional: true` today. That's why the rename waits for the ConfigMap.

**P2a.**
- Add `components/settings` with the `cluster-settings` ConfigMap (`CLUSTER_DOMAIN`,
  `CLUSTER_CLOUDFLARE_TUNNEL_ID`) and include it in every namespace `kustomization.yaml`,
  `external-secrets` included.
- Flip `cluster-settings` to `optional: false` in both places in `kubernetes/flux/cluster/ks.yaml`.
- Nothing references the new names yet.

**Gate:** Flux is Ready everywhere, and the ConfigMap exists in every namespace.

**P2b.** Rename `${SECRET_DOMAIN}` → `${CLUSTER_DOMAIN}` and `${SECRET_CLOUDFLARE_TUNNEL_ID}` →
`${CLUSTER_CLOUDFLARE_TUNNEL_ID}` in every manifest and in CLAUDE.md. `cluster-secrets` stays in
`substituteFrom`; its old names just go unused.

**Gate:** for every Kustomization the rename touches, the `flux build ks` output before and
after is byte-identical (`cmp`). That reads live substitutions, and the rendered output contains
no Secrets.

**Rollback:** revert P2b, then P2a.

## P3: namespaces, leaves first

Order:
1. home-assistant
2. o11y
3. yarr
4. rook-ceph
5. tailscale
6. volsync: the component (14 apps across yarr, attic, omada-system and home-assistant),
   plus `kopia-secret` and `volsync-maintenance-secret`
7. attic
8. network (24 h soak)
9. cert-manager (24 h soak)
10. flux-system (24 h soak)

### The standard two-commit flip

ESO adopts an existing Secret it doesn't own. Flux, in the same reconcile, applies first and
then prunes the old Secret, matching on its own labels, which ESO leaves in place. A one-commit
swap therefore deletes the Secret and ESO recreates it, leaving a gap. So each namespace flips
in two commits:

- **(A)** Add `kustomize.toolkit.fluxcd.io/prune: disabled` to the namespace's sops Secret(s).
  Gate: Flux is Ready, and the annotation is on the live Secret (`metadata.annotations`, read-only).
- **(B)** In one commit, remove each `secret.sops.yaml` and add `externalsecret.yaml` with the
  same `target.name`, `creationPolicy: Owner` (the default), `dataFrom.extract` per BWS secret,
  and a `target.template` keeping the old key names. Confirm the consumer carries the Reloader
  annotation. ESO adopts the Secret in place, with no gap.

Because key names never shrink, adoption leaves no stale keys behind.

### Per-namespace gate

1. Every new ExternalSecret shows `SecretSynced`.
2. Each target Secret's key names (`kubectl get secret … -o json | jq '.data | keys'`) equal the
   old sops file's key names.
3. Consumer HelmReleases are Ready, and pod restart counts haven't jumped.

### Exceptions

- **ghcr** (in yarr). Today it's copied into all 13 namespaces by `components/sops`.
  - In one commit (B), remove `ghcr-secret.sops.yaml` from `components/sops` and add the yarr
    ExternalSecret, with no step (A).
  - Flux prunes every copy, and ESO recreates the yarr one within seconds.
  - Only image pulls during that window would fail. A `prune: disabled` step would leave 12
    orphaned copies instead.
- **mktxp** (in o11y). mktxp v2.1.0 reads `username`/`password` from an external YAML file
  through `credentials_file`, and it overrides the inline values (`router_connection.py`). In
  one commit (B), with no step (A):
  - commit both `mktxp.conf` and `_mktxp.conf` as a ConfigMap `mktxp-config`, mounted at
    `/mktxp_config`, with the inline `username`/`password` removed and
    `credentials_file = /mktxp_secrets/credentials.yaml` in `[default]`;
  - add an ExternalSecret that writes the **new** Secret `mktxp-secret`, whose
    `credentials.yaml` contains `username: {{ .USERNAME | toJson }}` and
    `password: {{ .PASSWORD | toJson }}`. A JSON string is valid YAML, so special characters
    are safe;
  - mount that Secret as a directory at `/mktxp_secrets`, not through `subPath`, so a rotation
    shows up and Reloader restarts the pod;
  - repoint the HelmRelease. Flux prunes `mktxp-credentials` normally. A new name avoids
    adoption, which would otherwise leave the old `mktxp.conf` key, password included, in the
    adopted Secret.

  You do one manual step first: paste the current non-secret conf body into the ConfigMap
  without the two credential lines. Extra gate: mktxp `/metrics` returns router series.
- **attic** ([ADR-0016 addendum](./adr/0016-secret-delivery-bitwarden-sm-eso.md#addendum-2026-10-19-one-seed-not-two)).
  This uses the standard two commits.
  - In (B), `resources/server.toml` (still built by `configMapGenerator` as `attic-configmap`)
    swaps `${TAILSCALE_DOMAIN}` for `{{ .TAILNET }}`.
  - The ExternalSecret adds a `server.toml` key to `attic-secret` through `templateFrom`
    `configMap`, with `dataFrom` from `attic` and `tailscale`.
  - The `config-file` persistence moves from the ConfigMap to `attic-secret`, still at
    `/etc/attic/server.toml` through `subPath`. That's fine here, because Reloader restarts on
    change.
  - Key names only grow, so adoption is clean. Extra gate: attic answers on its tailnet
    hostname.

### Rollback

1. **By default, roll forward.** If a value is wrong, fix it in BWS, then add a
   `force-sync=<timestamp>` annotation to the ExternalSecret. Nothing is reverted.
2. **Real revert**, if ESO itself is broken:
   - first commit `kustomize.toolkit.fluxcd.io/prune: disabled` on the ExternalSecret.
     Pruning an Owner-mode ExternalSecret garbage-collects its Secret; this keeps the Secret.
   - Then revert (B), and the sops Secret takes over the same object.
3. For ghcr and mktxp, revert (B) and accept a gap of seconds.

## P4: deletion

Starts after a 7-day soak following flux-system.

**Pre-gate:**
- `git ls-files '*.sops.*'` lists only `components/sops/{cluster-secrets,sops-age}`,
  `talos/talsecret.sops.yaml` and `.sops.yaml`;
- every ExternalSecret is `SecretSynced`.

**P4a.**
- Remove `cluster-secrets` from `substituteFrom`: both entries in `kubernetes/flux/cluster/ks.yaml`,
  plus the redundant entries in `tailscale/operator`, `network/k8s-gateway`,
  `network/cloudflare-tunnel` and `kube-system/reloader` `ks.yaml`.
- Delete `cluster-secrets.sops.yaml`. Gate: Flux Ready, and a `flux build` spot-check on a few
  apps is unchanged.

**P4b.** Remove all three `decryption:` blocks: two in `kubernetes/flux/cluster/ks.yaml` and the
stray one in `kube-system/reloader/ks.yaml`. This must land before P4c. If Flux pruned `sops-age`
while a `decryption:` block still pointed at it, decryption would fail.
Gate: Flux is Ready everywhere. **This is the point of no return.**

**P4c.** Delete:
- `components/sops` and its entry in every namespace `kustomization.yaml`;
- `talos/talsecret.sops.yaml` and `.sops.yaml`;
- `.taskfiles/sops`, plus its include and `SOPS_*` vars in `Taskfile.yaml`;
- the `render-check` recipe and the `secrets_source` switch in `talos/mod.just`;
- `scripts/sops-metadata-check.sh`;
- `eso-canary` and the `canary` BWS secret;
- CLAUDE.md's sops guidance: the `task sops:encrypt` row, the SOPS gotcha, the
  `components/sops` gotcha and the architecture-tree line.

Gate:
- `git ls-files '*.sops.*'` is empty;
- `git grep -l 'ENC\['` is empty;
- `flux get ks -A` and `flux get hr -A` are all Ready.

**+30 days:** destroy `age.key`, both the repo-root copy on every workstation and the offline
copy in Keeper ▸ Homelab ▸ Break-glass. Until then it is the only way back into git history.
Cluster secrets aren't rotated, so history stays live for as long as the key exists.

## Appendix: backend checklist

To be turned into a `/wizard` script when the work starts.

1. **Org.** Create a Bitwarden **Free** organization for Secrets Manager, owned by a free
   account, separate from the Human Vault.
2. **Projects.** Create `homelab` and `bootstrap`. Leave the third free project spare.
3. **Machine accounts.**
   - `eso`: *Can read* on `homelab`.
   - `operator`: *Can read, write* on both projects.
4. **Tokens.**
   - Create an access token for each machine account.
   - Save the `operator` token in Keeper ▸ Homelab ▸ Break-glass.
   - Store the `eso` token as the `bootstrap` secret `eso-token`, and nowhere else.
5. **Secrets.** Create each secret in the Credential Sources tables above.
   - Values are JSON maps with the listed UPPER_SNAKE keys.
   - Copy values from a local `sops -d` of the matching file into the web UI.
   - `talsecret` holds the whole decrypted Talos bundle as YAML text.
   - `TAILNET` is the value currently in `cluster-secrets` `TAILSCALE_DOMAIN`.
6. **Keeper copies.** For `grafana`, `rook-ceph`, `omada`, `mikrotik` and `ghcr`, make sure
   the Keeper autofill copy matches.
7. **Gate.** Run the names-only listing from [Prep](#prep-populate-the-backend).
