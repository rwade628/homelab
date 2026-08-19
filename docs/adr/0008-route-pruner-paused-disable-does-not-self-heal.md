---
status: accepted
---

# Route pruner suspended: disabling a route doesn't self-heal like ADR 0004 assumed

The first live (non-dry-run) run of the route-pruning CronJob (ADR 0006/0007) executed successfully and reduced enabled routes as designed — `streaming-connector-1` 1747→874, `streaming-connector` 1522→761. But the Tailscale admin console then showed those disabled counts sitting in "awaiting approval," not gone. Investigating against the live API confirmed why: `advertisedRoutes` is completely immutable through the API (`GET /device/{id}/routes` still reports all 1747/1522 as advertised — unchanged), and `POST /device/{id}/routes` only ever controls which advertised routes are *enabled*. Disabling a route doesn't expire, forget, or remove it — it just reclassifies it as advertised-but-not-enabled, which the console renders as a pending approval.

ADR 0004 assumed this would be self-healing ("lets App Connector DNS interception relearn routes as real traffic resumes"). That assumption doesn't hold: a device only auto-approves a route the *first* time it advertises it. Since these IPs are already permanently in the device's advertised set (which has no expiry — ADR 0004's own finding), re-observing the same IP via DNS interception doesn't re-trigger anything; there's no "first advertise" event left to fire. A disabled route has no path back to being enabled short of manual approval in the console or a full re-registration of the device's identity (the "actual flush mechanism" ADR 0005 identified and explicitly deferred pending sign-off).

Net effect if left running: every monthly cycle disables ~50% of whatever's currently enabled, and none of the previously-disabled routes ever leave the pending pile, since nothing can remove them from `advertisedRoutes`. The enabled-route count keeps shrinking (the intended security benefit is real and does work), but the "awaiting approval" backlog grows without bound and never resolves itself.

We decided to suspend the CronJob (`cronjob.suspend: true` in `kubernetes/apps/tailscale/route-pruner/app/helmrelease.yaml`) rather than delete it, so the manifests and ADR trail are preserved while a redesign is decided. Separately, the pending routes from the one live run were manually reapproved in the admin console (both replicas back to fully enabled — see Consequences) to avoid the risk of streaming breakage while this is unresolved; the CronJob itself remains suspended regardless, since the underlying mechanism is still unfixed.

## Considered options

- **Accept the growing backlog as cosmetic** and keep running monthly — rejected for now, not ruled out permanently. The enabled-route reduction is real, but an unbounded, self-never-clearing pending pile in the admin console is exactly the kind of silent-until-it's-a-problem state ADR 0004 itself warned against ("the only way to notice if the job starts misbehaving").
- **Redesign around full re-auth** (delete a replica's `{pod, Secret}` together, per ADR 0005's deferred "actual flush mechanism") — not decided yet. This would give a genuinely clean slate (empty advertised set, not just empty enabled set) each cycle, matching ADR 0004's original self-healing intent far better than the disable-via-API approach does. Bigger change: touches device identity, and ADR 0005 already flagged its own auto-re-approval assumption as unverified.

## Consequences

- Production was manually reset to its pre-run state (`streaming-connector-1` and `streaming-connector` both back to 100% enabled, 0 pending, confirmed against the live API) — the one live run's route-count reduction was undone by hand rather than accepted, so there is currently no net effect from having run the job.
- Any future work on this job should re-litigate the pruning *mechanism* itself, not just tune the existing disable-based selection logic — the selection logic (ADR 0007) is sound, but the underlying action it drives doesn't achieve what ADR 0004 designed it to achieve.
