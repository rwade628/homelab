# Rook v1.20 / Kubernetes v1.36 Compatibility — Primary Source Verification

Research for issue [#1454](https://github.com/rwade628/homelab/issues/1454) ("Confirm v1.20
Kubernetes/Rook version compatibility"), a child of the wayfinder map issue
[#1450](https://github.com/rwade628/homelab/issues/1450) ("Rook-Ceph v1.20 Migration Map").

No existing `docs/research/` convention was found in this repo prior to this file
(`find -iname '*research*'` turned up nothing) — this file establishes the location per the task
instructions.

## Cluster's current state (from this repo)

- `talos/talconfig.yaml`: `kubernetesVersion: v1.36.1`
- `kubernetes/apps/rook-ceph/rook-ceph/app/ocirepository.yaml`: `ref.tag: v1.19.9`
- Target for migration: rook-ceph / rook-ceph-cluster chart `v1.20.5`

## Q1: Minimum/maximum supported Kubernetes versions for Rook v1.20.x

**Source A — Rook's official v1.20 upgrade guide**, raw source fetched directly from the
`release-1.20` branch (matches published page at
https://rook.io/docs/rook/v1.20/Upgrade/rook-upgrade/):
`https://raw.githubusercontent.com/rook/rook/release-1.20/Documentation/Upgrade/rook-upgrade.md`

Under "Breaking changes in v1.20":

> The minimum supported Kubernetes version is v1.31.

The upgrade guide states a floor only; it does not state a ceiling.

**Source B — Rook v1.20.0 GitHub release notes**
(https://github.com/rook/rook/releases/tag/v1.20.0, fetched via `gh api
repos/rook/rook/releases/tags/v1.20.0`):

Under "Features":

> Supported Kubernetes versions are v1.31 through v1.36.

This is the authoritative range statement (min **and** max) and is Rook's own documentation, not
a secondary source — it directly corroborates the "v1.31–v1.36" figure quoted in onedr0p/home-ops
PR #11548's body.

**Answer:** Rook v1.20.x officially supports **Kubernetes v1.31 through v1.36** (inclusive).

## Q2: Minimum Rook version required before upgrading to v1.20

**Source — same upgrade guide** (`Documentation/Upgrade/rook-upgrade.md`, release-1.20 branch):

Under "Supported Versions":

> This guide is for upgrading from **Rook v1.19.x to Rook v1.20.x**.

Under the "Helm" section, a stronger, version-specific floor is called out for Helm-based
installs (which is how this cluster installs rook-ceph, via `OCIRepository` + `HelmRelease`):

> !!! important
> Before upgrading to v1.20, ensure the cluster is already on at least v1.19.5.
> There was a critical update in [v1.19.5](https://github.com/rook/rook/releases/tag/v1.19.5)
> that will enable the helm upgrades to v1.20.

And further down, under "Rook Operator Upgrade":

> The examples given in this guide upgrade a live Rook cluster running `v1.19.10` to the version
> `v1.20.6`. This upgrade should work from any official patch release of Rook v1.19 to any
> official patch release of v1.20.

**Answer:** The documented floor is **Rook v1.19.5** (Helm-specific requirement, due to a critical
fix needed for the Helm upgrade path to succeed) — more generally, any v1.19.x patch release is
the supported upgrade path into v1.20.x.

## Q3: Does this cluster satisfy the documented bounds?

- **Kubernetes v1.36.1** (this cluster, per `talos/talconfig.yaml`) — Rook v1.20.x's documented
  range is v1.31–v1.36. v1.36.1 is a patch release within the v1.36 minor line, so it **falls
  within** the supported range, but **right at the upper edge** — it is the newest minor version
  Rook v1.20.x claims support for, with no headroom before the next Kubernetes minor bump would
  exceed Rook's stated ceiling.
- **Rook v1.19.9** (this cluster, per `kubernetes/apps/rook-ceph/rook-ceph/app/ocirepository.yaml`)
  — the documented floor to upgrade from is v1.19.5. v1.19.9 is a later patch than v1.19.5, so it
  **satisfies** the documented floor (and is well within "any v1.19.x patch release" more broadly).

**Bottom line: both preconditions are satisfied**, with the caveat that the Kubernetes version
sits at the top edge of Rook v1.20.x's supported range rather than comfortably in the middle —
worth flagging for the wayfinder map as a constraint on how much further Kubernetes can be bumped
before the next Rook upgrade is needed.

## Additional finding: CVE note in the v1.20 upgrade guide (relevant to picking v1.20.5)

The upgrade guide's "Breaking changes in v1.20" section also states:

> Be aware that **Ceph [CVE-2025–30156](https://ceph.io/en/news/blog/2026/v20-2-4-v19-2-6-combo-released/)**
> was announced. Rook users are advised to upgrade to Rook **v1.20.6 or higher**, and Ceph versions
> v20.2.4 or v19.2.6 as soon as possible. Ceph will report health errors until users rotate core
> CephX auth keys. ... Users upgrading from Rook v1.18 or lower should follow guidance in
> [Rook issue #18203](https://github.com/rook/rook/issues/18203#issuecomment-5397373786).

This cluster's migration target is chart **v1.20.5**, one patch below the v1.20.6-or-higher
recommendation tied to this CVE. Not one of the three questions this ticket asked, but directly
relevant to the wayfinder map's target-version choice and worth a follow-up decision (bump the
target to v1.20.6+, or confirm the CVE doesn't apply / plan a fast follow-up patch bump).

Note the guidance shifted between releases: the **v1.20.5 release notes themselves**
(https://github.com/rook/rook/releases/tag/v1.20.5, fetched via `gh api
repos/rook/rook/releases/tags/v1.20.5`) say:

> Rook users are advised to upgrade to Rook v1.20.5 or v1.19.9 and Ceph versions v20.2.4 or
> v19.2.6 as soon as possible.

— i.e., at the time v1.20.5 shipped, it was itself considered a sufficient CVE fix. The
**upgrade guide's currently-live text** (quoted above, fetched from the `release-1.20` branch
after v1.20.6 was released) has since been updated to say v1.20.6-or-higher. This is consistent,
not contradictory — the recommendation was tightened after v1.20.6 shipped with further hardening.
Since this cluster is targeting v1.20.5, it would satisfy the CVE guidance as it stood when
v1.20.5 was current, but not the guide's present-day recommendation.

## Secondary source consulted (context only, not used as the basis for the above)

**onedr0p/home-ops PR #11548** (https://github.com/onedr0p/home-ops/pull/11548), fetched via
`gh pr view 11548 --repo onedr0p/home-ops --json body`: its body states

> Prereqs: upgrade path requires coming from at least v1.19.5 (currently v1.19.9) and Kubernetes
> v1.31-v1.36 (currently v1.36.3). Ceph is HEALTH_OK.

This matches Rook's own documentation above (Sources A and B) — same "v1.19.5" floor, same
"v1.31–v1.36" range — and additionally shows the reference cluster upgraded from the same
Rook v1.19.9 this cluster is currently on, with a Kubernetes version (v1.36.3) one patch ahead of
this cluster's v1.36.1, both within the same v1.36 minor line. The reference cluster's account
checks out against the primary sources.

## Sources

- Rook v1.20 upgrade guide (published): https://rook.io/docs/rook/v1.20/Upgrade/rook-upgrade/
- Rook v1.20 upgrade guide (raw, release-1.20 branch, verified content):
  https://raw.githubusercontent.com/rook/rook/release-1.20/Documentation/Upgrade/rook-upgrade.md
- Rook v1.20.0 release notes: https://github.com/rook/rook/releases/tag/v1.20.0
- Rook v1.20.5 release notes: https://github.com/rook/rook/releases/tag/v1.20.5
- Rook v1.19.5 release notes (the referenced "critical update"):
  https://github.com/rook/rook/releases/tag/v1.19.5
- onedr0p/home-ops PR #11548 (secondary context): https://github.com/onedr0p/home-ops/pull/11548
