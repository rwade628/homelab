---
status: accepted
---

# VolSync Kopia cache reserves 90% of `VOLSYNC_CAPACITY`; size overrides for real headroom, defer full migration to kopiur

A `VolSyncVolumeOutOfSync` alert fired overnight for `qbittorrent` and never cleared. The `ReplicationSource`'s cache PVC (`volsync-src-qbittorrent-cache`, `2Gi`) was at 100% usage, and every hourly Kopia sync job since `02:01` failed identically with `error connecting to repository: ... no space left on device`. The live `qbittorrent` pod and its `/config` PVC were unaffected — this was a backup-pipeline failure, not data loss.

This cluster runs `ghcr.io/perfectra1n/volsync`, a community fork, because upstream `backube/volsync`'s Kopia mover support ([PR #1723](https://github.com/backube/volsync/pull/1723)) was never merged. The fork's `calculateCacheLimits()` auto-derives Kopia's `--metadata-cache-size-limit-mb` as 70% of the cache PVC's capacity and `--content-cache-size-limit-mb` as 20%, leaving exactly 10% of `VOLSYNC_CAPACITY` as real headroom for everything else (index files, sqlite locks, temp/log files). This 70/20 split is undocumented on the fork's own docs site — only visible in source comments — and Kopia's own upstream docs (kopia.io) publish no numeric defaults or headroom guidance for these flags either.

`qbittorrent`'s `VOLSYNC_CAPACITY` was set to `2Gi` (10% headroom = ~205MB) before this resource's history had migrated to Kopia (the live `ReplicationSource` status still carries a stale `restic.lastPruned` field) and was never revisited afterward. A repo-wide check found `home-assistant/mcp` at `1Gi` (10% headroom = ~100MB) — the same failure mode, not yet triggered. Every other one of the 13 apps using `kubernetes/components/volsync` is at `5Gi` or above (the component's own template default is `${VOLSYNC_CAPACITY:=5Gi}`) and none of those have failed.

## Considered options

- **Explicitly override `contentCacheSizeLimitMB`/`metadataCacheSizeLimitMB` to hand-tune fixed headroom.** Rejected for this fix: both fields exist on the fork's CRD, but every currently-healthy app in this cluster already relies on the plain 70/20 auto-split at `5Gi`+ capacity — introducing a second sizing convention for two apps would be a new pattern with no evidence it's needed over just matching the baseline everyone else already uses.
- **Migrate from VolSync to [kopiur](https://github.com/home-operations/kopiur)** (a newer Kopia-native operator from the `home-operations` org that onedr0p/home-ops has already fully migrated ~25+ apps to). Kopiur has no 70/20 auto-split — `metadataCacheSizeMb`/`contentCacheSizeMb` are independent, decoupled fields — so it eliminates this failure class entirely, and it ships a purpose-built `kubectl kopiur migrate volsync` tool that specifically detects `perfectra1n/volsync`'s Kopia mover and adopts the existing NFS Kopia repo (`10.0.10.3:/mnt/storage/k8s/kopia`) with snapshot history preserved, so migration would not require re-seeding backups. **Deferred, not rejected**: kopiur is explicitly self-described as alpha ("the CRD surface may still change between releases"), ~3 months old as of this writing. Committing all 13 apps' backup pipelines to an alpha CRD surface is a bigger, riskier change than tonight's alert warrants, even though it is well-precedented by onedr0p's own cluster. Tracked as a follow-up: [rwade628/homelab#1444](https://github.com/rwade628/homelab/issues/1444).

## Decision

Removed the `VOLSYNC_CAPACITY` overrides for `qbittorrent` (`2Gi`) and `home-assistant/mcp` (`1Gi`) entirely, letting both fall through to `kubernetes/components/volsync`'s own `5Gi` default — the same value every other healthy app in the cluster already uses, rather than inventing a new number.

## Consequences

- Sizing rule going forward: when setting `VOLSYNC_CAPACITY` for a new app on this fork, remember only ~10% of it is usable headroom beyond Kopia's own metadata/content cache reservation — don't size it to the source PVC's actual data volume, size it to leave real headroom (`5Gi` has proven sufficient for every app so far; smaller values need explicit justification).
- The full VolSync→kopiur migration remains open as future work — kopiur is worth revisiting once it has more of a production track record (post-alpha, more releases behind it). Migration should be low-risk when it happens, given the repo-preserving migration tool already exists.
- The two stuck job pods' retry loops for `qbittorrent` clear on their own once a sync succeeds against the resized cache; no manual cleanup was required.
