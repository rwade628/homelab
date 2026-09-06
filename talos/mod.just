# talos/mod.just — invoked as `just talos <recipe>` via the root .justfile's `mod talos`

export TALOSCONFIG := "clusterconfig/talosconfig"

endpoint := "https://10.0.10.245:6443"
hostnames := "ser8a ser8b ser8c"

# Render a node's full machine config to stdout
[private]
render hostname:
    #!/usr/bin/env bash
    set -euo pipefail
    talos_version=$(yq '.talosVersion' versions.yaml)
    kubernetes_version=$(yq '.kubernetesVersion' versions.yaml)
    schematic=$(yq '.schematicId' versions.yaml)
    talosctl gen config homelab {{endpoint}} \
      --with-secrets <(sops -d talsecret.sops.yaml) \
      --kubernetes-version "${kubernetes_version}" \
      --install-image "factory.talos.dev/installer/${schematic}:${talos_version}" \
      --additional-sans 127.0.0.1,10.0.10.245 \
      --output-types controlplane -o - \
    | talosctl machineconfig patch /dev/stdin \
        -p @cluster.yaml \
        -p @controlplane.yaml \
        -p @nodes/{{hostname}}.yaml

# Diff a node's rendered config against its live running config (secrets/generated-only fields noisy — review by eye)
diff hostname:
    diff <(just talos render {{hostname}}) <(talosctl -n {{hostname}} get machineconfig -o yaml)

# Apply a node's rendered config. mode: no-reboot|auto|reboot
apply-config hostname mode="no-reboot":
    just talos render {{hostname}} | talosctl apply-config --mode={{mode}} --nodes {{hostname}} --file /dev/stdin

# Upgrade Talos on a single node. rollout=true skips the Flux suspend/resume (caller already did it)
upgrade-node hostname rollout="false":
    #!/usr/bin/env bash
    set -euo pipefail
    talos_version=$(yq '.talosVersion' versions.yaml)
    [ "{{rollout}}" = "true" ] || flux --namespace flux-system suspend kustomization --all
    schematic=$(kubectl get node {{hostname}} --output=jsonpath='{.metadata.annotations.extensions\.talos\.dev/schematic}')
    talosctl --nodes {{hostname}} upgrade --image="factory.talos.dev/installer/${schematic}:${talos_version}" --timeout=10m
    talosctl --nodes {{hostname}} health --wait-timeout=10m --server=false
    [ "{{rollout}}" = "true" ] || flux --namespace flux-system resume kustomization --all

# Upgrade Talos across the whole cluster, one node at a time
upgrade-cluster:
    #!/usr/bin/env bash
    set -euo pipefail
    flux --namespace flux-system suspend kustomization --all
    for hostname in {{hostnames}}; do
      just talos upgrade-node "$hostname" true
    done
    flux --namespace flux-system resume kustomization --all

# Upgrade the Kubernetes version
upgrade-k8s version:
    talosctl --nodes $(talosctl config info --output json | jq --raw-output '.endpoints[]' | shuf -n 1) upgrade-k8s --to {{version}}

# Reset all nodes back to maintenance mode (destructive)
[confirm('This will destroy your cluster and reset the nodes back to maintenance mode. Continue? [y|N]')]
reset *args:
    #!/usr/bin/env bash
    set -euo pipefail
    for hostname in {{hostnames}}; do
      talosctl --nodes "$hostname" reset --reboot {{args}} --graceful=false --wait=false
    done
