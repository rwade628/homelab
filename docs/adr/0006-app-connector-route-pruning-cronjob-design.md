---
status: accepted
---

# App connector route-pruning CronJob: capped reset, deterministic selection, Go stdlib

The `networking` repo's ADR 0004 mandates a scheduled Kubernetes CronJob, running in this cluster, that prunes the `streaming` app connector's (`tag:hbo-connector`/`tag:netflix-connector`, see `kubernetes/apps/tailscale/operator/app/connectors.yaml`) accumulated routes on a monthly basis. That ADR settles the *policy* (reset monthly, cap removals at 50% per run, log to Grafana, use the narrowest available API credential) but leaves the *implementation* open. This ADR records the choices made building it, since several are non-obvious and would otherwise look arbitrary to a future reader.

We decided:

- **Capped reset, not full wipe.** ADR 0004's own text is internally inconsistent — the main body says the job "disables every currently-enabled route" while its Consequences section says removals must be capped at 50% per run. We treat the capped version as the real spec: it has an explicit safety rationale (a single bad `POST` can otherwise wipe a connector's entire route list) that the "disable everything" phrasing lacks.
- **Deterministic, not audit-log-informed, route selection for phase 1.** Tailscale's Configuration Audit Log retains 90 days of route-approval events with per-CIDR timestamps, which could rank routes oldest-first for the cap. We chose not to build this yet, for two reasons: it's exactly the option ADR 0004 explicitly deferred ("revisit after roughly three reset cycles"), and it wouldn't help on the first run anyway — these connectors have accumulated routes uninterrupted for 286+ days (`0005-app-connector-pod-restart-does-not-flush-routes.md`), so almost the entire current backlog predates the 90-day audit window and would sort as equally "unknown-age." The script still logs each disabled route's audit-log timestamp, if one exists, purely so real churn data accumulates for a future revisit — but it doesn't act on it.
- **Go, standard library only, run via `go run` against a ConfigMap-mounted script.** This repo has no image-build pipeline (`CLAUDE.md`: pure manifest repo, every app pulls a pre-published image). Using Tailscale's official `tailscale-client-go` SDK would require a `go.mod`/`go.sum`, which forces a `go mod download` against the public module proxy on every pod start — a new failure mode and trust surface for a job that runs unattended and can mutate live routing state. A single Go file with zero third-party imports needs no `go.mod` at all, so `go run main.go` works with no runtime dependency beyond the Tailscale API itself, at the cost of hand-rolling the HTTP/JSON instead of using the typed SDK.
- **Generic `tag:*-connector` matching, not hardcoded tags.** Enumerates target devices by pattern rather than naming `hbo-connector`/`netflix-connector` explicitly, so a future third app connector doesn't require a code change here.
- **Per-device failure isolation.** The two `streaming` replicas are pruned independently; one device's API failure doesn't block the attempt on the other (they don't fail over for each other — ADR 0005). The job as a whole exits non-zero, surfacing failure via the normal Job-failure alert path, if any device failed.
- **Dedicated, narrowly-scoped credential.** A new OAuth client/secret, separate from the operator's `operator-oauth`, scoped to `devices:routes` (write) + `devices:core:read` (list, needed for tag enumeration — there's no server-side tag filter on the device-list endpoint). Note `devices:routes` doesn't support Tailscale's tag-restriction feature (only `devices:core`/`auth_keys`/`all` do), so this credential is tailnet-wide for routes even though it's only ever exercised against connector-tagged devices in practice.
- **`--dry-run` flag.** Computes and logs the intended change without calling `POST`, so the job can be safely exercised via `kubectl create job --from=cronjob` before trusting an unattended monthly run against a destructive endpoint.

## Considered options

- **Full reset every run** (`POST {"routes": []}`) — rejected; see above, this is what the 50%-cap consequence in ADR 0004 exists to prevent.
- **Audit-log-informed oldest-first selection now** — deferred, not rejected. Revisit once a few reset cycles have run and the audit log actually reflects post-reset churn rather than 286+ days of pre-reset backlog.
- **Go with the official `tailscale-client-go` SDK** — rejected for now. The typed-request safety it offers wasn't judged worth a runtime dependency on the Go module proxy for a job whose whole design otherwise minimizes trust surface (narrow OAuth scope, dry-run, per-device isolation).
- **Python, standard library only** — genuinely comparable to the Go-stdlib choice (both avoid a build pipeline and runtime third-party dependencies); Go was picked on maintainer preference, not a technical deciding factor.

## Consequences

- Phase 1 ships with no real staleness signal — the 50% cap removes an arbitrary, deterministically-ordered half of a connector's routes each run, not necessarily the least-used half. This is expected to be revisited using the audit-log timestamps this job already logs.
- Because the OAuth credential's `devices:routes` scope can't be tag-restricted, it technically has route-write access to every device in the tailnet, not just connector-tagged ones. The generic `tag:*-connector` device enumeration in the job's own logic is the only thing constraining its blast radius — a bug there would not be caught by the credential's scope.
- Adding a Go dependency later (e.g. to eventually adopt the official SDK) means introducing a `go.mod`/`go.sum` and accepting the runtime module-fetch behavior this ADR avoided — that trade-off should be revisited explicitly, not slipped in incidentally.
