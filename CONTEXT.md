# Homelab

GitOps-managed homelab Kubernetes cluster (Talos + Flux). This glossary only covers terms that have needed pinning down during design discussions — it is not a full domain model of every app in the cluster.

## Language

**Alert Entity**:
A dedicated Home Assistant entity (`binary_sensor.alert_*` or `input_boolean.alert_*`) whose sole purpose is to be flipped by an HA automation and exposed through the HomeKit Bridge as a trip-wire. It carries no control semantics of its own — the actual notification is fired by an Apple Home/Shortcuts automation on the Apple TV hub that watches the entity, not by HA. See [ADR-0001](./docs/adr/0001-homekit-bridge-for-apple-home-notifications.md).
_Avoid_: notification entity, trigger sensor (too generic — this is specifically the HA-side half of a two-system handoff, not a general-purpose trigger)

## Related vocabulary (owned by sibling repos)

- **Trusted Zone / Restricted Zone / Pinhole** — network segmentation vocabulary for VLANs, defined in the `mikrotik` repo's `CONTEXT.md`. Referenced but not redefined here; see [ADR-0002](./docs/adr/0002-macvlan-dual-homing-for-cross-vlan-device-discovery.md) for how this cluster's workloads interact with those zones.
