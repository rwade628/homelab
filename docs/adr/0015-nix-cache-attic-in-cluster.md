# Nix binary cache (Attic) runs in-cluster, PVC-backed, exposed via Tailscale

Ryan's personal Nix package cache (previously an Incus LXC on TrueNAS running Harmonia) moves
into this cluster as **Attic**, cache name `fafnir` — see `dot.nix`'s
`docs/adr/0005-nix-cache-moves-to-cluster-ci-builds-all-platforms.md` for why. It's exposed over
Tailscale, matching the existing pattern for Jellyfin/Channels/Grafana, so `idun` (a roaming Mac)
can pull from it whether it's home or away, alongside plain in-cluster/LAN access. GitHub Actions
runners (which build and push to the cache — see the `dot.nix` ADR) join the tailnet for the job
via the official `tailscale/github-action` rather than exposing a second push path through the
existing Cloudflare Tunnel, keeping read and write access on one consistent network model.

Storage is split across two plain Rook-Ceph RBD (`ceph-block`) PVCs, not one: a small `attic`
volume holding only the sqlite database — provisioned the same way every other Volsync-backed app
in this cluster gets its main volume, via `kubernetes/components/volsync`'s `${APP}` PVC, not a
hand-written manifest — covered by the existing Volsync/Kopia backup pipeline to the NAS like
every other stateful app here, and a larger, unbacked `attic-storage` volume (a plain
`PersistentVolumeClaim` manifest) holding the actual nar/chunk blob store. This deviates from this
cluster's usual "back up the whole PVC" convention deliberately — the blob store is fully
rebuildable from `dot.nix`'s flake, so losing it in a cluster rebuild just costs a rebuild, not
data, while backing up tens of GB of substitutable cache blobs on every Kopia run would buy
nothing but backup-storage cost. The sqlite DB is small and holds the metadata (tokens, cache
config, chunk index) that isn't reconstructible from the blobs alone, so it gets the normal
treatment. Not worth standing up S3/CephObjectStore for the sole purpose of this cache, either.
`ceph-block` itself has ample headroom for this (794GiB free cluster-wide at time of writing) and
is already the established pattern for large, unbacked, cache-like volumes here (`jellyfin-cache`
and `stash-cache` are 50Gi/25Gi `ceph-block` PVCs on the same "not worth backing up" basis).

Garbage collection is Attic's built-in `default-retention-period` (30 days, set declaratively in
`server.toml`): objects untouched for 30 days become eligible for deletion, and any pull resets
the clock. This is a last-accessed/LRU mechanism, not literal reference-counting against
`dot.nix`'s `flake.lock` — the two converge in practice, since machines stop pulling a superseded
lockfile's paths and those age out within 30 days, but a pinned older revision that's still
occasionally built against would survive under real reference-counting while remaining eligible
for GC here if unused for 30 days. That gap is accepted as fine for a personal cache; literal
lockfile-diffing would need separate tooling Attic doesn't provide.

## Consequences

- Attic has no GitOps-native way to declare caches, permissions, or tokens — the `fafnir` cache
  and its push/pull tokens are created imperatively, once, via `atticadm` exec'd into the running
  pod (per the issue's own "manual login/push/pull round-trip" acceptance criterion). This is
  undeclared state living only in the server's sqlite DB, not in this repo. It only needs redoing
  after catastrophic loss of the `attic` PVC (the DB is Volsync-backed, so this should be rare),
  not on every reconcile or redeploy.
- The push-scoped token minted for `dot.nix`'s CI is handed off out-of-band (GitHub Actions
  secret, optionally noted in a password manager) — it is never committed to this repo, sops-
  encrypted or otherwise, since it's a credential for another repo's CI, not cluster state.
