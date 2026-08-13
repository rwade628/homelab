# Homelab

GitOps-managed homelab Kubernetes cluster (Talos + Flux). This glossary only covers terms that have needed pinning down during design discussions — it is not a full domain model of every app in the cluster.

## Language

**GPU Share**:
One of N concurrent claims on a node's DRM render node (`/dev/dri/renderD*`), advertised as the extended resource `devic.es/dri-render`. Deliberately *not* an exclusive allocation: several pods hold a share of the same physical iGPU simultaneously and the kernel time-slices between them. A request for `devic.es/dri-render: 1` therefore expresses "this workload needs a GPU node and counts against a concurrency ceiling", not "this workload owns the GPU". See [ADR-0003](./docs/adr/0003-reject-amd-gpu-operator-for-integrated-graphics.md).
_Avoid_: GPU, `amd.com/gpu` (the vendor operator's name for the same device, which does imply exclusivity — and names a vendor rather than the kernel interface actually being shared)

**Alert Entity**:
A dedicated Home Assistant entity (`binary_sensor.alert_*` or `input_boolean.alert_*`) whose sole purpose is to be flipped by an HA automation and exposed through the HomeKit Bridge as a trip-wire. It carries no control semantics of its own — the actual notification is fired by an Apple Home/Shortcuts automation on the Apple TV hub that watches the entity, not by HA. See [ADR-0001](./docs/adr/0001-homekit-bridge-for-apple-home-notifications.md).
_Avoid_: notification entity, trigger sensor (too generic — this is specifically the HA-side half of a two-system handoff, not a general-purpose trigger)

## Related vocabulary (owned by sibling repos)

- **Trusted Zone / Restricted Zone / Pinhole** — network segmentation vocabulary for VLANs, defined in the `mikrotik` repo's `CONTEXT.md`. Referenced but not redefined here; see [ADR-0002](./docs/adr/0002-macvlan-dual-homing-for-cross-vlan-device-discovery.md) for how this cluster's workloads interact with those zones.
- **AMF / Bundled ffmpeg / Terminal pin** — vocabulary for what actually consumes a GPU Share inside the Channels DVR container, defined in the `packages` repo's `CONTEXT.md`. Relevant here because [ADR-0003](./docs/adr/0003-reject-amd-gpu-operator-for-integrated-graphics.md)'s `/dev/kfd` finding is a constraint on that image, and because Channels' hardware encoding is AMF-only — VA-API is structurally unavailable to it, so a working Jellyfin VA-API path implies nothing about Channels.
