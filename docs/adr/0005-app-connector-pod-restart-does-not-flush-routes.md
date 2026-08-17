---
status: accepted
---

# Restarting a Tailscale app connector pod does not flush its learned routes

The `streaming` app connector (`tag:hbo-connector`/`tag:netflix-connector`, 2 replicas) accumulates learned `/32` routes from observed DNS lookups for its configured domains (`netflix.com`, `*.netflix.com`, `hbomax.com`, `*.hbomax.com`, `max.com`, `*.max.com`) with no built-in expiry. After 286+ days the two replicas had diverged badly — 1522 routes on one, 909 on the other — and there's no `tailscale acl test`-style CLI for inspecting or pruning this state (`tailscale debug appc` doesn't exist as a subcommand on the operator's bundled client, v1.102.2).

We tested whether a plain pod restart (`kubectl delete pod ts-streaming-2g6p6-1`) resets this accumulated state, on the theory that it might be in-memory/ephemeral. It does not: the route count was identical (909) before and after the restart, including after a 15s wait for netmap propagation. The reason is visible in the pod spec — `tailscaled`'s config and node identity are persisted via a Kubernetes Secret (`ts-streaming-2g6p6-<hash>`) mounted back in on every restart, so the control plane recognizes the recreated pod as the *same* node with the *same* learned-route history. Pod lifecycle and connector route state are decoupled.

The other replica (1522 routes) was left untouched throughout this test and kept serving, so there was no streaming interruption.

## Considered options

- **Delete the pod's backing Secret along with the pod**, forcing genuine re-registration (new node key, empty route set). This is the actual flush mechanism, not tested yet — deferred pending explicit sign-off, since it tears down and recreates the device's tailnet identity (not just its process), which is a more consequential action than a bounce and hasn't been exercised even once.
- **Live with the accumulated/diverged routes.** Streaming works today; the cost is diffuse (each replica separately over-broad on shared CDN IP ranges — see the `networking` repo's ADR 0003 for the app-connector-vs-CDN-overlap discussion) rather than an active failure. Accepted as the status quo for now.

## Consequences

- If route hygiene on this connector becomes worth doing, the real lever is deleting `{pod, Secret}` together per replica, not a plain restart — restarting alone accomplishes nothing here and could be mistaken for having worked, since nothing errors, it just silently doesn't do anything.
- Re-authing a replica this way should re-approve routes automatically (`tag:hbo-connector`/`tag:netflix-connector` are in the tailnet policy's `autoApprovers.routes`), so it shouldn't need manual admin-console approval -- but this is inferred from the ACL policy, not yet confirmed by actually doing it.
- Whatever mechanism is used, do it one replica at a time (as done here) so the other keeps serving -- there's no indication these two replicas fail over for each other automatically if both went down simultaneously.
