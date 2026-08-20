---
name: cluster-bootstrap-and-new-apps
description: How to scaffold a new app under kubernetes/apps/<ns>/<app>/, and the legacy makejinja/helmfile bootstrap tasks for standing up a cluster from scratch. Use when adding a new app to the cluster or when asked about task init/configure/bootstrap:talos/bootstrap:flux/workstation:venv.
---

## Adding a new app

Add a `<namespace>/<app>/` directory following the `ks.yaml` + `app/kustomization.yaml` + `app/helmrelease.yaml` + `app/ocirepository.yaml` pattern, then add `./<app>/ks.yaml` to the namespace's `kustomization.yaml` resources list. Copy an existing similar app (e.g. `kubernetes/apps/o11y/gatus`) as a template rather than starting from scratch.

## Bootstrap tasks (legacy — cluster already exists, rarely needed)

`task init`, `task configure`, `task bootstrap:talos`, `task bootstrap:flux`, `task workstation:venv` render `config.yaml` via makejinja and stand up a cluster from scratch. Don't run these against the live cluster unless deliberately re-bootstrapping.
