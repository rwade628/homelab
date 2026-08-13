---
status: accepted
---

# HomeKit Bridge as the path for Apple Home notifications, not direct push

Home Assistant already has a working direct-push path (`notify.mobile_app_ryans_iphone` via the iOS Companion App integration) that could deliver any alert without touching HomeKit at all. We decided against it for the notification pipeline: alerts should surface as native Apple Home notifications instead, so household members without the HA app installed still see them, and so they show up through Apple's own notification system (Apple Watch, HomeKit-aware devices) rather than being tied to one person's Companion App.

HomeKit itself has no "send this text as a notification" primitive — the only way to get there is HA exposing a trip-wire [Alert Entity](../../CONTEXT.md) through the already-live `homekit:` bridge (`HASS Bridge:21063`), and a separate automation authored in the Apple Home app (or Shortcuts) on the Apple TV hub that watches that entity and fires the actual notification.

This means "complex automation in HA" stops at the trip-wire — the automation *logic* (thresholds, timing, conditions) lives in HA, but the notification *firing* is a second automation living in Apple Home, outside version control and outside this repo.

## Considered options

- **`notify.mobile_app_*` direct push** — simplest, fully within HA, fully in this repo. Rejected because it doesn't produce native Apple Home notifications and only reaches one person's device.
- **New/separate HomeKit Bridge instance just for alerts** — cleaner separation between controllable accessories and notification trip-wires, but doubles the pairing/management overhead in Apple Home. Rejected in favor of adding Alert Entities to the existing bridge's `include_entities` filter.

## Consequences

- New alerts require two changes in two systems: an HA automation + Alert Entity (this repo), and an Apple Home/Shortcuts automation (not in this repo, not version-controlled).
- The existing bridge's `include_entities` filter (`kubernetes/apps/home-assistant/home-assistant/app` config) is the single registration point for both real accessories and Alert Entities — don't split them into a second bridge without revisiting this decision.
