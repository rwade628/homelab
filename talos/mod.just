# talos/mod.just — invoked as `just talos <recipe>` via the root .justfile's `mod talos`

export TALOSCONFIG := "clusterconfig/talosconfig"

hostnames := "ser8a ser8b ser8c"

# Render a node's full machine config to stdout
[private]
render hostname:
    #!/usr/bin/env bash
    set -euo pipefail
    # cluster.yaml/controlplane.yaml are a hand-authored base (see ADR-0014, not talosctl gen
    # config) with secret fields as $VAR placeholders, filled in here from talsecret.sops.yaml via
    # yq's built-in envsubst operator (avoids a new dependency on GNU gettext's envsubst, which
    # isn't guaranteed to be installed). Substituted content is written to real temp files with a
    # .yaml suffix — talosctl's patch-type detection needs a recognizable extension, which a bare
    # process-substitution fd (/dev/fd/N) doesn't have.
    secrets=$(sops -d talsecret.sops.yaml)
    export MACHINE_CA_CRT=$(yq -r '.certs.os.crt' <<< "$secrets")
    export MACHINE_CA_KEY=$(yq -r '.certs.os.key' <<< "$secrets")
    export MACHINE_TOKEN=$(yq -r '.trustdinfo.token' <<< "$secrets")
    export CLUSTER_CA_CRT=$(yq -r '.certs.k8s.crt' <<< "$secrets")
    export CLUSTER_CA_KEY=$(yq -r '.certs.k8s.key' <<< "$secrets")
    export CLUSTER_AGGREGATOR_CA_CRT=$(yq -r '.certs.k8saggregator.crt' <<< "$secrets")
    export CLUSTER_AGGREGATOR_CA_KEY=$(yq -r '.certs.k8saggregator.key' <<< "$secrets")
    export CLUSTER_ETCD_CA_CRT=$(yq -r '.certs.etcd.crt' <<< "$secrets")
    export CLUSTER_ETCD_CA_KEY=$(yq -r '.certs.etcd.key' <<< "$secrets")
    export CLUSTER_SERVICEACCOUNT_KEY=$(yq -r '.certs.k8sserviceaccount.key' <<< "$secrets")
    export CLUSTER_TOKEN=$(yq -r '.secrets.bootstraptoken' <<< "$secrets")
    export CLUSTER_ID=$(yq -r '.cluster.id' <<< "$secrets")
    export CLUSTER_SECRET=$(yq -r '.cluster.secret' <<< "$secrets")
    export CLUSTER_SECRETBOX_ENCRYPTION_SECRET=$(yq -r '.secrets.secretboxencryptionsecret' <<< "$secrets")
    export TALOS_VERSION=$(yq '.talosVersion' versions.yaml)
    export KUBERNETES_VERSION=$(yq '.kubernetesVersion' versions.yaml)
    export TALOS_SCHEMATIC=$(yq '.schematicId' versions.yaml)

    base=$(mktemp --suffix=.yaml)
    cp_patch=$(mktemp --suffix=.yaml)
    trap 'rm -f "$base" "$cp_patch"' EXIT

    yq e '(.. | select(tag == "!!str")) |= envsubst' cluster.yaml > "$base"
    yq e '(.. | select(tag == "!!str")) |= envsubst' controlplane.yaml > "$cp_patch"

    talosctl machineconfig patch "$base" \
        -p @"$cp_patch" \
        -p @nodes/{{hostname}}.yaml

# Diff a node's rendered config against its live running config
diff hostname:
    just talos render {{hostname}} | talosctl apply-config --dry-run --nodes {{hostname}} --file /dev/stdin

# Apply a node's rendered config. mode: no-reboot|auto|reboot
apply-config hostname mode="no-reboot":
    just talos render {{hostname}} | talosctl apply-config --mode={{mode}} --nodes {{hostname}} --file /dev/stdin

# Reboot a single node, cordoning/draining first to avoid pod-scheduling churn during shutdown
reboot hostname:
    #!/usr/bin/env bash
    set -euo pipefail
    kubectl cordon {{hostname}}
    kubectl drain {{hostname}} --ignore-daemonsets --delete-emptydir-data --timeout=5m
    talosctl --nodes {{hostname}} reboot
    talosctl --nodes {{hostname}} health --wait-timeout=10m --server=false
    kubectl uncordon {{hostname}}

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
