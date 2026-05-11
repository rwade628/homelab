# Homelab → onedr0p/home-ops Migration Plan

This document outlines the migration path from this homelab repository to align with [onedr0p/home-ops](https://github.com/onedr0p/home-ops).

## Comparison: homelab vs onedr0p/home-ops

### 1. Namespace Architecture

| Local (homelab) | home-ops | Notes |
|---|---|---|
| `yarr/` | `default/` | home-ops consolidates ALL user apps into `default` |
| `home-assistant/` | `default/home-assistant` | Moved to default |
| `omada-system/` | *(none)* | home-ops uses UniFi; you'd use Omada |
| `observability/` | `o11y/` | Renamed |
| *(none)* | `actions-runner-system/` | Missing |
| *(none)* | `external-secrets/` | Missing |
| *(none)* | `system-upgrade/` | Missing |
| `amd/` | *(none)* | Your AMD GPU namespace |
| `tailscale/` | *(none)* | Your Tailscale setup |

**Key insight**: home-ops uses a flat `default` namespace for most user-facing apps. Your `yarr/` namespace would need to be merged into `default/`.

### 2. Networking Stack

| Component | Local | home-ops | Notes |
|---|---|---|---|
| Cloudflare DNS updater | `cloudflare-dns/` | `cloudflare-dns/` | Same |
| Cloudflare Tunnel | `cloudflare-tunnel/` | `cloudflare-tunnel/` | Same |
| Envoy Gateway | `envoy-gateway/` | `envoy-gateway/` | Same |
| **Internal DNS** | `k8s-gateway/` | *(removed)* | home-ops uses Cloudflare Tunnel only |
| **External DNS** | *(none)* | `unifi-dns/` | home-ops uses UniFi webhook |
| Certificates | embedded in cert-manager ns | `certificates/` (dedicated) | Structural change |
| Echo server | *(none)* | `echo/` | Health check endpoint |

**Critical difference**: home-ops removed k8s-gateway entirely, relying on Cloudflare Tunnel for both ingress and internal DNS. You **cannot** do this since you use Mikrotik (not UniFi). You need k8s-gateway + an external DNS updater for Mikrotik.

### 3. Observability Stack

| Component | Local | home-ops | Notes |
|---|---|---|---|
| Metrics backend | `kube-prometheus-stack` | `victoria-metrics/` | **Major change** |
| Logging | `loki/` + `promtail/` | `victoria-logs/` | **Major change** |
| Grafana | `grafana/` | `grafana/` | Same |
| Prometheus operator CRDs | *(none)* | `prometheus-operator-crds/` | home-ops splits this out |
| Node exporter | *(none)* | `node-exporter/` | home-ops has it |
| Kube state metrics | *(none)* | `kube-state-metrics/` | home-ops has it |
| Blackbox exporter | *(none)* | `blackbox-exporter/` | home-ops has it |
| SNMP exporter | *(none)* | `snmp-exporter/` | Useful for Mikrotik! |
| Smartctl exporter | *(none)* | `smartctl-exporter/` | home-ops has it |
| Unpoller | *(none)* | `unpoller/` | UniFi only - skip for you |
| Gatus | *(none)* | `gatus/` | Status page |
| Kromgo | *(none)* | `kromgo/` | Status badges |
| Prometheus adapter | *(none)* | `prometheus-adapter/` | HPA support |
| Silence operator | *(none)* | `silence-operator/` | Alert management |
| mktxp | `mktxp/` | *(none)* | Your Mikrotik tool |

### 4. Components Layer

| Component | Local | home-ops |
|---|---|---|
| `common/` (repos + sops + ns) | Yes | **No** - split into namespace-level patterns |
| `volsync/` | Yes | Yes |
| `alerts/` | *(none)* | Yes - Kustomize Component |
| `zeroscaler/` | *(none)* | Yes - scale-to-zero for low-traffic apps |

### 5. Default Namespace Apps (home-ops has, you don't)

`agregarr`, `atuin`, `autobrr`, `brrpolice`, `deduparr`, `go2rtc`, `mosquitto`, `notifier`, `plex`, `qui`, `sabnzbd`, `seasonpackerr`, `seerr`, `slskd`, `smtp-relay`, `thelounge`, `zigbee`, `zwave`

### 6. Secret Management

| | Local | home-ops |
|---|---|---|
| Method | SOPS + age | **External Secrets + 1Password** |

This is a **major architectural difference**. home-ops uses External Secrets Operator with 1Password Connect for secret management. Your setup uses SOPS which is simpler and doesn't require a paid service.

---

## Migration Plan

### Phase 1: Observability — Victoria Metrics Migration (Highest Impact)

This is the biggest change. home-ops migrated from Prometheus stack to Victoria Metrics.

**Steps:**
1. Remove `kube-prometheus-stack`, `loki`, `promtail` from `observability/`
2. Add `victoria-metrics/`, `victoria-logs/`, `prometheus-operator-crds/`
3. Add `node-exporter/`, `kube-state-metrics/`, `blackbox-exporter/`, `smartctl-exporter/`
4. Add `snmp-exporter/` — **this is valuable for Mikrotik monitoring**
5. Add `gatus/` for status page, `kromgo/` for status badges
6. Add `silence-operator/` for alert management
7. Add `prometheus-adapter/` for HPA support
8. Rename namespace from `observability` to `o11y`
9. Update Grafana datasources to point to Victoria Metrics
10. Update dashboards/alerts for VM-compatible metrics

**Consider keeping**: `mktxp` if you want Mikrotix-specific monitoring.

### Phase 2: Networking — Align While Keeping Mikrotik Support

**Steps:**
1. **Keep** `k8s-gateway/` — you need it (no UniFi)
2. **Add** `certificates/` as a dedicated namespace for ClusterIssuers (move from cert-manager)
3. **Add** `echo/` for health check endpoint
4. **Research Mikrotik DNS updater** — see options below
5. Remove `k8s-gateway/` only if you later adopt a different internal DNS strategy

**Mikrotik DNS Updater Options:**
- **`ddns-updater`** (github.com/ioverb/ddns-updater) — supports Mikrotik as a provider
- **`cloudflare-ddns`** + custom DNS API calls — script-based approach
- **`ddns-updater`** (github.com/favonia/cloudflare-dns) — supports multiple providers including dynamic DNS APIs
- **Custom script** using Mikrotik's REST API or WinBox-compatible endpoints
- **`mikro-dns-updater`** — community tools specifically for Mikrotik DDNS

The `ddns-updater` by ioverb supports many providers and may have Mikrotik or similar dynamic DNS support.

### Phase 3: Components Layer Restructure

**Steps:**
1. Create `components/alerts/` — Kustomize Component with alert resources (Alertmanager config, GitHub status)
2. Create `components/zeroscaler/` — scale-to-zero component for low-traffic apps
3. Keep `components/volsync/` as-is
4. Remove `components/common/` — home-ops doesn't use a common component; instead each namespace `kustomization.yaml` directly includes resources

### Phase 4: App Consolidation (default namespace)

**Steps:**
1. Create `default/` namespace with `kustomization.yaml`
2. Move apps from `yarr/` → `default/`
3. Move `home-assistant/` → `default/home-assistant`
4. Consider moving `matter-server` to `default/` or keep separate
5. Add new apps from home-ops as desired: `mosquitto`, `zigbee`, `zwave`, `smtp-relay`, `thelounge`, `go2rtc`, `seerr`, etc.

### Phase 5: Add Missing home-ops Namespaces

1. **`actions-runner-system/`** — Self-hosted GitHub Actions runners
2. **`system-upgrade/`** — System Upgrade Controller for Talos/K8s upgrades
3. **`external-secrets/`** — Only if you want to migrate from SOPS to 1Password

### Phase 6: kube-system Alignment

home-ops has `descheduler` and `intel-gpu-resource-driver` (for Intel GPUs). You have AMD GPU, so:
- Add `descheduler/` — helps optimize pod scheduling
- Keep `amd/` namespace for GPU operator
- Remove `csi-driver-nfs` if not needed (home-ops doesn't have it — uses NFS differently)

---

## Hardware-Driven Deviations (Cannot Follow home-ops)

| home-ops Feature | Why You Can't Use It | Alternative |
|---|---|---|
| `unifi-dns/` | No UniFi — you have Mikrotik | k8s-gateway + Mikrotik DDNS |
| `unpoller/` | UniFi-only | `snmp-exporter/` for Mikrotik SNMP |
| `intel-gpu-resource-driver/` | You have AMD GPU | `amd/` namespace with gpu-operator |
| `external-secrets/` (1Password) | Requires 1Password ($65/yr) | Keep SOPS + age (free) |
| `actions-runner-system/` | Requires GitHub Actions setup | Optional |

---

## Recommended Priority Order

1. **Observability migration** (Victoria Metrics) — biggest structural change, highest value
2. **Components layer** (alerts, zeroscaler) — enables future apps
3. **Networking** (certificates app, echo, Mikrotik DNS) — aligns structure while keeping your hardware
4. **Default namespace consolidation** — merge yarr/home-assistant into default
5. **Add missing namespaces** (system-upgrade, actions-runner)
6. **New apps** — add selectively based on needs (gatus, kromgo, smtp-relay, etc.)
