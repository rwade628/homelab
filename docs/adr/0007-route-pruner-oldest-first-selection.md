---
status: accepted
---

# Route pruner selection upgraded to real oldest-first, ahead of schedule

ADR 0006 shipped the route-pruning CronJob with deterministic (arbitrary CIDR-order) selection for its 50% cap, explicitly deferring audit-log-informed oldest-first selection — reasoning that it was unproven and, per ADR 0004, meant to wait for "roughly three reset cycles" of real data.

Deploying and live-testing that ADR immediately surfaced three real bugs in the audit-log integration (wrong response shape assumed by ADR 0004, a decode failure on heterogeneous event types, and a device-ID mismatch — the audit log keys events on a device's `nodeId`, not its numeric `id`). Fixing all three and re-testing against production data gave a clean result: of 3269 currently-enabled routes across both `streaming` replicas, every single one resolved to either a real first-approval timestamp or a definite "predates the 90-day window" classification — zero unresolved. That's a materially different risk picture than what ADR 0006 was reasoning about (an unproven mechanism), so we're pulling the upgrade forward rather than waiting out the originally planned three cycles.

We decided: `pruneDevice` now ranks each device's enabled routes oldest-first before applying the 50% cap, instead of an arbitrary deterministic order. Routes with no precise age data (predate the audit window, or have no matching event at all) are treated as the oldest tier and are prioritized for removal, since there's no positive evidence they're recent; routes with a known first-approval timestamp are removed oldest-timestamp-first, and are protected from removal ahead of any route with unknown age. Ties within a tier break on the CIDR string for a stable, reproducible order run to run.

## Considered options

- **Wait out the three cycles as ADR 0006 planned** — rejected. That plan was written to avoid designing against a guess; the guess has since been replaced with a validated result against the exact production data this job operates on.

## Consequences

- The cap now genuinely protects recently-approved routes over old ones, which is the actual goal of pruning — ADR 0006's cap was a blast-radius limiter with no preference about *which* half survived.
- Selection quality is bounded by the 90-day audit log window: on a connector with 292+ days of accumulated routes, most currently-enabled routes still fall in the "unknown age" tier and are ordered arbitrarily (by CIDR) relative to each other within that tier. This improves automatically over time as the backlog is pruned and more of the route set falls within a window the audit log can actually see.
