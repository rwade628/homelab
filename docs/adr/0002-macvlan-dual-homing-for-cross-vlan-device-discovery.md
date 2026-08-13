---
status: accepted
---

# Macvlan dual-homing across VLANs for local device discovery, instead of routed L3

Home Assistant needs reliable mDNS/SSDP discovery of and low-latency operational sessions with devices that live outside its own VLAN. mDNS/SSDP don't cross VLANs via routing — only via an explicit repeater (the mikrotik router's `mdns-repeat-ifaces`) or by being L2-adjacent to the target broadcast domain. We chose the latter: Home Assistant is dual-homed via Multus macvlan directly onto both `HOME_VLAN` (`net1`) and `IOT_VLAN` (`net2`), each with its own pinned MAC and IP, rather than living on one VLAN and relying on routing plus mDNS-repeat.

`matter-server` will get the same second leg onto `IOT_VLAN`, since [device placement policy](#device-placement) is moving IoT-ish devices there and Matter leans on mDNS for operational discovery just as heavily as HA's other local integrations do.

The mikrotik repo's Trust Zone model (`HOME_VLAN` = Trusted, `IOT_VLAN` and friends = Restricted, cross-zone traffic gated by explicit Pinholes — see that repo's `CONTEXT.md` and ADR-0001) governs *routed* traffic between VLANs. Macvlan dual-homing sits outside that model entirely: because a pod's `IOT_VLAN` interface is L2-switched onto that VLAN's own broadcast domain, its traffic to `IOT_VLAN` devices never traverses the router's forward chain, so the firewall's zone policy doesn't see it. In effect, any workload dual-homed this way becomes its own cross-zone trust boundary — if the Home Assistant pod were compromised, it would already have direct L2 reach into `IOT_VLAN` regardless of what the firewall allows. This is accepted as the cost of reliable local discovery; it is not a gap to "fix" with firewall rules, since firewall rules can't see this traffic at all.

## Considered options

- **Single interface on `HOME_VLAN` + router `mdns-repeat` + routed L3 to `IOT_VLAN`** — no extra interfaces to manage, and stays entirely inside the router's zone policy. Rejected as the primary mechanism: `mdns-repeat` only leaks *presence*, not full multicast/operational reliability, and the mikrotik ADR itself calls this out as the coarser tradeoff, not a substitute for a real connection.
- **Route everything, no macvlan** — simplest from the network's point of view, but breaks local push/mDNS-dependent integrations that expect same-subnet discovery (this is why the `home`/`iot` NetworkAttachmentDefinitions exist at all). Rejected.

## Consequences

- Any workload dual-homed onto a Restricted Zone VLAN via macvlan is a deliberate, accepted trust boundary — new dual-homed workloads should be added consciously, not by default, since each one is a potential pivot point the router firewall cannot mediate.
- A companion fix lives in the `mikrotik` repo (separate git repository): the live `"Allow VLAN to VLAN"` firewall rule currently accepts all inter-VLAN traffic unconditionally, which doesn't match that repo's own accepted Trust Zone ADR. Tightening it to enforce default-deny + Pinholes (including a new `IOT_VLAN → Apple TV` Pinhole for Matter/HomeKit) is tracked there, not here — this ADR only covers what's inside this repo's control.
