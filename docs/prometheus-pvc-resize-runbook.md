# Prometheus PVC resize runbook (50 Gi → 120 Gi)

**Why:** Prometheus retention went from 30d to 90d (rwade628/workbench#24). That needs about 100 GB, so the HelmRelease now asks for a 120 Gi volume with `retentionSize: 110GB`. Kubernetes can't grow a volume through a StatefulSet, and the Prometheus Operator doesn't resize existing PVCs. So the PVC has to be grown by hand, once.

**Source:** this follows the Prometheus Operator's "Resizing volumes" procedure (`Documentation/platform/storage.md`): pause the Prometheus, set the new size, grow each PVC, orphan-delete the StatefulSet, unpause. The pause and the unpause go through git. The chart renders `spec.paused: false`, so a pause set by hand would be undone by the next Helm upgrade.

**Who:** a human runs the `kubectl` steps. Agents don't run `kubectl patch` or `delete` against this cluster (see CLAUDE.md).

## 1. Commit A (already made): pause, plus the new size

`kube-prometheus-stack` values now set `paused: true`, `retention: 90d`, `retentionSize: 110GB` and `storage: 120Gi`. Push it and let Flux apply it.

While it's paused, Prometheus keeps running and scraping. The operator, though, does nothing: new ServiceMonitors (including `wifi-probe`) and rule changes aren't picked up until step 4. Do steps 2–4 soon. The PVC is still 50 Gi, and `retentionSize` is larger than that.

```sh
export KUBECONFIG=~/git/homelab/kubeconfig
NS=o11y
PVC=prometheus-kube-prometheus-stack-db-prometheus-kube-prometheus-stack-0

# Both print the new values: true, then 120Gi.
kubectl -n $NS get prometheus kube-prometheus-stack -o jsonpath='{.spec.paused}{"\n"}'
kubectl -n $NS get prometheus kube-prometheus-stack \
  -o jsonpath='{.spec.storage.volumeClaimTemplate.spec.resources.requests.storage}{"\n"}'
```

## 2. Grow the PVC (by hand)

`ceph-block` allows expansion, and RBD resizes online.

```sh
kubectl -n $NS patch pvc $PVC -p '{"spec":{"resources":{"requests":{"storage":"120Gi"}}}}'
kubectl -n $NS get pvc $PVC -w      # wait until CAPACITY shows 120Gi, then Ctrl-C
```

## 3. Orphan-delete the StatefulSet (by hand)

The pod keeps running, and no StatefulSet is recreated while the operator is paused.

```sh
kubectl -n $NS delete statefulset prometheus-kube-prometheus-stack --cascade=orphan
```

## 4. Commit B: unpause

Delete the `paused: true` line, and its TEMPORARY comment, from `kubernetes/apps/o11y/kube-prometheus-stack/app/helmrelease.yaml`, then commit and push. The operator recreates the StatefulSet with the 120 Gi template, adopts the running pod, and resumes normal reconciling.

## Verify

```sh
kubectl -n $NS get prometheus kube-prometheus-stack -o jsonpath='{.spec.paused}{"\n"}'   # false
kubectl -n $NS get sts prometheus-kube-prometheus-stack                                    # 1/1 ready
kubectl -n $NS get pvc $PVC                                                                 # 120Gi
```

In Prometheus, `kubelet_volume_stats_capacity_bytes{persistentvolumeclaim="prometheus-kube-prometheus-stack-db-prometheus-kube-prometheus-stack-0"}` should show about 128.8e9, and `up{job="wifi-probe"}` should be 1.

## If something goes wrong

- **The PVC stays at 50Gi:** run `kubectl -n $NS describe pvc $PVC` and look for resize events. If an RBD resize is stuck, the CSI driver logs live in `rook-ceph`.
- **No StatefulSet after step 4:** confirm that `spec.paused` is false and that the HelmRelease reconciled the commit (`flux get hr -n o11y kube-prometheus-stack`).
