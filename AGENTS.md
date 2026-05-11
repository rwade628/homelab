# Homelab Cluster Template — Agent Guide

## What This Repo Is

Talos Linux + Flux GitOps Kubernetes cluster. `config.yaml` (from `config.sample.yaml`) feeds [makejinja](https://github.com/mirkolenz/makejinja) to render all Kubernetes manifests from Jinja2 templates.

**Toolchain**: `task` (go-task), `.mise.toml` for tool versions, Python venv for makejinja.

## Daily Commands

| Task | What it does |
|------|-------------|
| `task kubernetes:reconcile` | Force Flux to pull latest Git changes |
| `task kubernetes:apply-ks PATH=<ns>/<app>` | Apply a single Flux Kustomization (e.g. `network/echo-server`) |
| `task kubernetes:resources` | Dump all common cluster resources (for debugging/support) |
| `task talos:upgrade-cluster` | Upgrade Talos on all nodes |
| `task talos:upgrade-k8s` | Upgrade Kubernetes on controller |
| `task talos:upgrade-node HOSTNAME=...` | Upgrade Talos on a single node |
| `task talos:reset [--force]` | Wipe cluster, reset nodes to maintenance mode |
| `task sops:encrypt` | Encrypt all unencrypted `*.sops.*` files under `kubernetes/` |

## Bootstrap (legacy)

These tasks exist from the initial setup but are no longer needed for day-to-day ops:

| Task | What it does |
|------|-------------|
| `task init` | Copy `config.sample.yaml` → `config.yaml` |
| `task configure` | Render templates + encrypt SOPS secrets + kubeconform validation |
| `task bootstrap:talos` | Bootstrap Talos cluster |
| `task bootstrap:flux` | Deploy Flux CRDs + sync cluster to Git repo |
| `task workstation:venv` | Create Python venv (required for makejinja) |
| `task workstation:direnv` | Run `direnv allow .` |

## Architecture

```
kubernetes/
  flux/
    cluster/ks.yaml   ← cluster-meta → cluster-apps (dependsOn)
    meta/             ← HelmRepositories, OCIRepositories
  apps/               ← one dir per namespace (kube-system, observability, network, etc.)
    <ns>/<app>/
      ks.yaml         ← Flux Kustomization
      app/kustomization.yaml
      app/helmrelease.yaml   ← usually app-template chart via OCIRepository
  components/         ← reusable snippets (volsync, sops-age, repos)
talos/                ← talconfig.yaml, talenv.yaml, talsecret.sops.yaml
bootstrap/
  scripts/plugin.py   ← makejinja plugin (custom filters/functions, default values)
```

**Flux flow**: `cluster/ks.yaml` → `kubernetes/flux/meta` (repos) → `kubernetes/apps` (per-app HelmReleases). SOPS decryption and `cluster-secrets`/`cluster-settings` substitution are applied top-down via patches.

## Gotchas

- **Jinja2 uses custom delimiters**: blocks are `#% ... %#`, variables are `#{ ... }#`. Standard `{{ }}` will NOT work in templates.
- **SOPS encryption**: all `*.sops.*` files under `kubernetes/` and `talos/` must be encrypted. The `.sops.yaml` uses `mac_only_encrypted` mode. After editing any SOPS file, run `task sops:encrypt`.
- **`config.yaml` is gitignored** — it's generated from `config.sample.yaml` via `task init`. Never commit it.
- **`age.key`** is the SOPS/Flux secret key. Keep it safe. It's gitignored.
- **`task configure` prompts** before overwriting `kubernetes/` — confirm only when you intend a full re-render.
- **After `task talos:reset`**, you must run `task bootstrap:talos` again (it checks for existing secrets and skips generation if present).
- **Apps use `app-template` chart** from OCIRepository (`ghcr.io/bjw-s/helm-charts`). The HelmRelease references it via `chartRef.kind: OCIRepository` — not a traditional HelmRepository + chart reference.
- **`kubeconfig` and `talosconfig` are gitignored** — they're generated at bootstrap time.
- **`.taskfiles/User/Taskfile.yaml` is optional** — if it exists, its tasks are included.

## Debugging

1. `flux get ks -A` / `flux get hr -A` — check Flux resources
2. `kubectl -n <ns> get pods -o wide` → `kubectl -n <ns> logs <pod>`
3. `kubectl -n <ns> describe <resource> <name>`
4. `task kubernetes:resources` — full cluster snapshot
5. `stern -n <ns> <fuzzy>` — tail multiple pod logs

## Style

- YAML: 2-space indent. Python/Bash: 4-space indent.
- HelmReleases use `chartRef` (OCIRepository) pattern, not legacy `chart.repository/chart.name`.
- All apps follow `<namespace>/<app>/ks.yaml + app/helmrelease.yaml` structure.
