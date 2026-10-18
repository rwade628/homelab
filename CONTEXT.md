# Homelab

GitOps-managed homelab Kubernetes cluster (Talos + Flux). This glossary only covers terms that have needed pinning down during design discussions — it is not a full domain model of every app in the cluster.

## Language

**GPU Share**:
One of N concurrent claims on a node's DRM render node (`/dev/dri/renderD*`), advertised as the extended resource `devic.es/dri-render`. Deliberately *not* an exclusive allocation: several pods hold a share of the same physical iGPU simultaneously and the kernel time-slices between them. A request for `devic.es/dri-render: 1` therefore expresses "this workload needs a GPU node and counts against a concurrency ceiling", not "this workload owns the GPU". See [ADR-0003](./docs/adr/0003-reject-amd-gpu-operator-for-integrated-graphics.md).
_Avoid_: GPU, `amd.com/gpu` (the vendor operator's name for the same device, which does imply exclusivity — and names a vendor rather than the kernel interface actually being shared)

**Fafnir**:
The single Attic cache namespace this cluster serves — Attic's internal per-cache permission and
retention scope, not the Attic server itself or the Kubernetes PVCs (`attic`, `attic-storage`)
that back it. See [ADR-0015](./docs/adr/0015-nix-cache-attic-in-cluster.md).
_Avoid_: "the cache" (ambiguous between the Attic cache namespace, the Attic server as a whole,
and the underlying PVCs)

**Alert Entity**:
A dedicated Home Assistant entity (`binary_sensor.alert_*` or `input_boolean.alert_*`) whose sole purpose is to be flipped by an HA automation and exposed through the HomeKit Bridge as a trip-wire. It carries no control semantics of its own — the actual notification is fired by an Apple Home/Shortcuts automation on the Apple TV hub that watches the entity, not by HA. See [ADR-0001](./docs/adr/0001-homekit-bridge-for-apple-home-notifications.md).
_Avoid_: notification entity, trigger sensor (too generic — this is specifically the HA-side half of a two-system handoff, not a general-purpose trigger)

**AP**:
A TP-Link Omada access point (the EAP225v3s, the EAP775-Wall) adopted by the in-cluster Omada controller. It is the thing that degrades and gets rebooted when Wi-Fi turns bad — distinct from the edge router (the RB5009), which is not involved in that failure mode.
_Avoid_: router, "the routers" (in casual use this has meant the APs; the actual router is a different device owned by the `mikrotik` repo)

**Secrets Stack**:
The pair of products that hold every secret: a **Human Vault** and a **Machine-Secrets Backend**. Chosen as Keeper (human) + Bitwarden Secrets Manager (machine).
_Avoid_: "the vault" (ambiguous between the two halves)

**Human Vault**:
The password manager people use directly: logins, passkeys, notes, family sharing. It is Keeper, on the sponsored Family plan. The cluster never reads from it.
_Avoid_: password manager (when the machine half could be meant), Keeper vault (names the vendor, not the role)

**Machine-Secrets Backend**:
The store the cluster reads secrets from through External Secrets. It is Bitwarden Secrets Manager, in its own free Bitwarden organization, separate from the Human Vault. See [ADR-0016](./docs/adr/0016-secret-delivery-bitwarden-sm-eso.md).
_Avoid_: KSM (a Keeper product this stack ruled out), secret store (ESO's `SecretStore` is the in-cluster object pointing *at* the backend, not the backend)

**Credential Source**:
The external service or system a secret belongs to (Cloudflare, Tailscale, the Kopia repository), as opposed to the app that consumes it. The Machine-Secrets Backend holds one secret per Credential Source, and several apps can read the same one.
_Avoid_: app secret, per-app secret (that names the consumer, and one Credential Source often serves several)

**Bootstrap Seed**:
A Secret written into the cluster by the operator's CLI during bootstrap, before ESO can deliver anything. It lives in no file and isn't managed by Flux. There are exactly two: ESO's root credential and the tailnet name.
_Avoid_: bootstrap secret (too broad, since the Talos bundle is also needed at bootstrap but is never written into the cluster)

**Root Account**:
An account that can recover a vault: the primary email and the Apple account. Its second factor and backup codes never depend on the Human Vault, because recovering the Human Vault depends on it.
_Avoid_: main account, master account (says nothing about the recovery dependency)

## Related vocabulary (owned by sibling repos)

- **Trusted Zone / Restricted Zone / Pinhole** — network segmentation vocabulary for VLANs, defined in the `mikrotik` repo's `CONTEXT.md`. Referenced but not redefined here; see [ADR-0002](./docs/adr/0002-macvlan-dual-homing-for-cross-vlan-device-discovery.md) for how this cluster's workloads interact with those zones.
- **AMF / Bundled ffmpeg / Terminal pin** — vocabulary for what actually consumes a GPU Share inside the Channels DVR container, defined in the `packages` repo's `CONTEXT.md`. Relevant here because [ADR-0003](./docs/adr/0003-reject-amd-gpu-operator-for-integrated-graphics.md)'s `/dev/kfd` finding is a constraint on that image, and because Channels' hardware encoding is AMF-only — VA-API is structurally unavailable to it, so a working Jellyfin VA-API path implies nothing about Channels.
- **Host / Platform / Host secrets file** — vocabulary for how Ryan's machines are provisioned, defined in the `dot.nix` repo's `CONTEXT.md`. Relevant here because [ADR-0015](./docs/adr/0015-nix-cache-attic-in-cluster.md)'s Attic cache serves builds produced from that flake.
