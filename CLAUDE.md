# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repo Is

A GitOps-managed homelab Kubernetes cluster: **Talos Linux** nodes + **Flux** (via `flux-operator`/`FluxInstance`, not `flux bootstrap`) syncing this repo. This is a live, running cluster (`rwade628/homelab`, domain `casadewade.com`) — it started from [onedr0p/cluster-template](https://github.com/onedr0p/cluster-template) but has diverged; treat the `bootstrap/` tooling (makejinja/helmfile) as legacy/one-time and the actual `kubernetes/` and `talos/` trees as the source of truth for how things currently work.

**Toolchain**: `task` (go-task) as the command runner. CLI tools are provided by the workstation's global Nix environment (managed outside this repo) — there is no per-repo tool manifest.

## Commonly Used Commands

| Task | What it does |
|------|-------------|
| `task kubernetes:reconcile` | Force Flux to pull latest Git changes (`flux reconcile ks cluster --with-source`) |
| `task kubernetes:apply-ks PATH=<ns>/<app> [NS=<namespace>]` | Force-apply a single Flux Kustomization from local disk, e.g. `PATH=o11y/gatus` |
| `task kubernetes:kubeconform` | Validate all rendered manifests under `kubernetes/` with kubeconform (also runs in CI) |
| `task kubernetes:resources` | Dump nodes/gitrepositories/kustomizations/helmreleases/certs/ingresses/pods (debugging snapshot) |
| `task talos:upgrade-cluster` | Rolling Talos upgrade across all nodes (suspends Flux, upgrades each node, resumes) |
| `task talos:upgrade-node HOSTNAME=<node>` | Upgrade Talos on one node |
| `task talos:upgrade-k8s` | Upgrade the Kubernetes version via `talosctl upgrade-k8s` |
| `task talos:apply-config HOSTNAME=<node> [MODE=no-reboot\|auto\|reboot]` | Push regenerated Talos machine config to a node |
| `task talos:reset [--force]` | Wipe the cluster back to maintenance mode (destructive, confirms first) |
| `task sops:encrypt` | Encrypt every `*.sops.*` file under `kubernetes/` that isn't already encrypted |

There is no app-level build/test/lint step — this is a pure manifest repo. "Testing" a change means: render/validate it with kubeconform, and/or `flux build` it locally before applying.

### Validating a single app before pushing

```sh
kustomize build kubernetes/apps/<ns>/<app>/app --load-restrictor=LoadRestrictionsNone | \
  kubeconform -strict -ignore-missing-schemas -skip Secret \
    -schema-location default \
    -schema-location 'https://kubernetes-schemas.pages.dev/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'
```

Or build what Flux would actually apply (resolves `postBuild` substitutions against the live cluster):

```sh
flux build ks <app> --path kubernetes/apps/<ns>/<app>/app --kustomization-file kubernetes/apps/<ns>/<app>/ks.yaml
```

### Bootstrap tasks (legacy — cluster already exists, rarely needed)

See the `cluster-bootstrap-and-new-apps` skill.

## Architecture

```
kubernetes/
  flux/cluster/ks.yaml     ← the ONE thing flux-instance points at (path: kubernetes/flux/cluster);
                              this is the "cluster-apps" Kustomization that fans out to kubernetes/apps,
                              with patches injecting decryption/postBuild/retry defaults onto every child KS/HR
  apps/<namespace>/
    kustomization.yaml     ← namespace-level: sets `namespace:`, includes components (sops, alerts), lists ./<app>/ks.yaml
    namespace.yaml
    <app>/
      ks.yaml               ← Flux Kustomization: path ./kubernetes/apps/<ns>/<app>/app, targetNamespace <ns>
      app/
        kustomization.yaml  ← resources: helmrelease.yaml, ocirepository.yaml, extras (prometheusrule, httproute, secret.sops.yaml...)
        helmrelease.yaml    ← HelmRelease using chartRef -> OCIRepository (NOT chart.spec.sourceRef)
        ocirepository.yaml  ← pins the chart OCI tag (this is what Renovate bumps)
  components/               ← reusable Kustomize components mixed into namespace kustomizations
    sops/                   ← cluster-secrets.sops.yaml, sops-age.sops.yaml, ghcr-secret.sops.yaml
    alerts/                 ← Alertmanager alert/provider wiring
    volsync/                ← PVC + ReplicationSource/Destination snippets for backup-enabled apps
talos/                      ← talconfig.yaml (talhelper), talenv.yaml, talsecret.sops.yaml, clusterconfig/ (generated, gitignored except talosconfig)
bootstrap/                  ← one-time cluster bring-up (makejinja templates, helmfile, kustomize) — legacy path, not day-to-day
scripts/                    ← kubeconform.sh (CI validation), bootstrap-apps.sh, plugin.py (makejinja custom filters)
```

**Flux flow**: `flux-operator` (HelmRelease in `apps/flux-system/flux-operator`) reconciles a `FluxInstance` (`apps/flux-system/flux-instance`) whose `spec.sync.path` is `kubernetes/flux/cluster`. That single `cluster-apps` Kustomization applies `kubernetes/apps` and stamps every discovered child Kustomization/HelmRelease with shared defaults (SOPS decryption, `cluster-secrets`/`cluster-settings` substitution, retry/remediation strategy) via `spec.patches` — so individual app `ks.yaml`/`helmrelease.yaml` files stay minimal and don't repeat this boilerplate.

**Adding a new app**: see the `cluster-bootstrap-and-new-apps` skill.

**Key platform components** (what's actually deployed, not template defaults):
- **Networking**: Cilium (CNI + LB IPAM), **Envoy Gateway** (Gateway API — `Gateway`/`HTTPRoute`, not ingress-nginx/Ingress) with two Gateways in `network` ns: `envoy-external` (public, `external.${SECRET_DOMAIN}`) and `envoy-internal` (LAN-only, `internal.${SECRET_DOMAIN}`). Apps attach via `route.<name>.parentRefs` in their HelmRelease values.
- **DNS/TLS**: `cloudflare-dns` (external-dns), `cloudflare-tunnel`, `cert-manager`, `k8s-gateway` for split-horizon internal DNS, Tailscale operator.
- **Storage**: Rook-Ceph (`ceph-block` StorageClass — used by most stateful apps) plus OpenEBS. Volsync (via `components/volsync`) handles PVC backup/replication for apps that opt in.
- **Observability (`o11y` namespace)**: kube-prometheus-stack, Grafana, victoria-logs, Gatus (status page), blackbox/snmp/smartctl exporters, silence-operator, kromgo, prometheus-adapter.
- **Apps use the `app-template` chart** (`oci://ghcr.io/bjw-s-labs/helm/app-template`) via `OCIRepository` + `HelmRelease.spec.chartRef` almost universally — values follow bjw-s app-template conventions (`controllers`, `service`, `route`, `persistence`, `defaultPodOptions`, etc.).

## Gotchas

- **Jinja2 uses custom delimiters** in `bootstrap/` templates: blocks are `#% ... %#`, variables are `#{ ... }#` (see `makejinja.toml`). Standard `{{ }}` will NOT work there — but note this only affects the legacy `bootstrap/` template tree, not the live `kubernetes/` manifests.
- **SOPS encryption**: every `*.sops.*` file under `kubernetes/` and `talos/` must stay encrypted. `.sops.yaml` uses `mac_only_encrypted` mode. After hand-editing any `*.sops.*` file, run `task sops:encrypt` before committing — never commit an unencrypted secret.
- **`age.key`**, **`kubeconfig`**, and **`talos/clusterconfig/talosconfig`** are the live cluster credentials — all gitignored. Never print their contents or commit them.
- **HelmReleases use `chartRef` (OCIRepository)**, not the legacy `chart.spec.sourceRef`/`chart.spec.chart` pattern — Renovate bumps the `OCIRepository.spec.ref.tag`, not a `HelmRepository` version.
- **Namespace-level `kustomization.yaml` files are the registry** — a new app under `apps/<ns>/<app>/` does nothing until its `ks.yaml` is added to `apps/<ns>/kustomization.yaml`'s `resources`.
- **`kubernetes/flux/cluster/ks.yaml` patches are load-bearing** for every app — don't duplicate `decryption`/`postBuild.substituteFrom`/retry settings in individual app manifests; they're injected centrally.
- **`components/sops` must be included** in a namespace's `kustomization.yaml` (`components:`) for that namespace's SOPS secrets to decrypt correctly.
- **Renovate** (`.renovaterc.json5`) drives most day-to-day changes (chart/image bumps via PR) — expect frequent small commits like `feat(container): update <chart> group (x.y.z ➔ x.y.z)`; match that style for similar automated-looking bumps.

## Debugging

1. `flux get ks -A` / `flux get hr -A` / `flux get sources oci -A` — check Flux Kustomizations, HelmReleases, OCIRepositories
2. `kubectl -n <ns> get pods -o wide` → `kubectl -n <ns> logs <pod>`
3. `kubectl -n <ns> describe <resource> <name>` / `kubectl -n <ns> get events --sort-by='.metadata.creationTimestamp'`
4. `task kubernetes:resources` — full cluster snapshot
5. `stern -n <ns> <fuzzy>` — tail multiple pod logs

## Style

- YAML: 2-space indent. Python/Bash: 4-space indent (`.editorconfig`).
- Every manifest starts with a `# yaml-language-server: $schema=...` comment pointing at the matching CRD/core-resource schema — keep this when adding new resources (copy from a sibling file of the same `kind`).
- All apps follow `<namespace>/<app>/ks.yaml` + `app/kustomization.yaml` + `app/helmrelease.yaml` structure; don't invent a different layout.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (`rwade628/homelab`), via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context layout: `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
