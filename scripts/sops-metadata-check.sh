#!/usr/bin/env bash
# Metadata-only facts for the zero-sops cutover (#1649): which sops values are equal to each
# other, and the key *names* inside multi-line configs. Decrypts in memory only; every line of
# output is an allowlisted boolean or a name that passed a strict regex, so a bug fails closed.
# Delete with the rest of the sops tooling in the final cutover phase.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
export SOPS_AGE_KEY_FILE="${SOPS_AGE_KEY_FILE:-$PWD/age.key}"

A=kubernetes/apps
declare -A F=(
    [seed]=bootstrap/kustomize/apps/network/secret.sops.yaml
    [certmgr]=$A/cert-manager/cert-manager/app/secret.sops.yaml
    [cfdns]=$A/network/cloudflare-dns/app/secret.sops.yaml
    [cftun]=$A/network/cloudflare-tunnel/app/secret.sops.yaml
    [tsop]=$A/tailscale/operator/app/secret.sops.yaml
    [tsrp]=$A/tailscale/route-pruner/app/secret.sops.yaml
    [kopia]=$A/volsync-system/kopia/app/secret.sops.yaml
    [vsmaint]=$A/volsync-system/volsync/maintenance/secret.sops.yaml
    [vscomp]=kubernetes/components/volsync/secret.sops.yaml
    [recyc]=$A/yarr/recyclarr/app/secret.sops.yaml
    [hook]=$A/yarr/webhook/app/secret.sops.yaml
    [fluxwh]=$A/flux-system/flux-instance/app/secret.sops.yaml
    [konfl]=$A/flux-system/konflate/app/secret.sops.yaml
    [grafana]=$A/o11y/grafana/app/secret.sops.yaml
    [rook]=$A/rook-ceph/rook-ceph/app/secret.sops.yaml
    [mktxp]=$A/o11y/mktxp/app/secret.sops.yaml
    [ghcr]=kubernetes/components/sops/ghcr-secret.sops.yaml
)

# Decrypted JSON lives only in this associative array, never on disk or in argv.
declare -A D
for k in "${!F[@]}"; do
    D[$k]=$(sops -d --output-type json "${F[$k]}")
done

# v <file-alias> <key>  →  jq expression input for one value (stringData, else base64-decoded data)
jqv='def v($k): (.stringData[$k] // ((.data // {})[$k] | if . then @base64d else null end));'

# bool <label> <jq-filter> <alias...>: run filter over the slurped docs; print only true/false.
bool() {
    local label=$1 filter=$2; shift 2
    local out
    out=$(for a in "$@"; do printf '%s\n' "${D[$a]}"; done | jq -s -r "$jqv $filter" 2>/dev/null || true)
    [[ "$out" == true || "$out" == false ]] || out=ERROR
    printf '%-48s %s\n' "$label" "$out"
}

# names <label> <jq-filter> <alias>: print names only, each must match a conservative regex.
names() {
    local label=$1 filter=$2 alias=$3 n
    printf '%s\n' "$label"
    printf '%s\n' "${D[$alias]}" | jq -r "$jqv $filter" 2>/dev/null | while IFS= read -r n; do
        if [[ "$n" =~ ^[A-Za-z0-9_.:\ -]{1,64}$ ]]; then printf '    %s\n' "$n"; else printf '    <rejected>\n'; fi
    done
}

echo "## Equality"
bool "cloudflare: cert-manager == cloudflare-dns token" '(.[0]|v("api-token")) as $x | .[1]|v("api-token") == $x' certmgr cfdns
bool "cloudflare: network seed == tunnel TUNNEL_TOKEN" '(.[0]|v("TUNNEL_TOKEN")) as $x | .[1]|v("TUNNEL_TOKEN") == $x' seed cftun
bool "tailscale: operator == route-pruner client id" '(.[0]|v("client_id")) as $x | .[1]|v("TS_OAUTH_CLIENT_ID") == $x' tsop tsrp
bool "tailscale: operator == route-pruner secret" '(.[0]|v("client_secret")) as $x | .[1]|v("TS_OAUTH_CLIENT_SECRET") == $x' tsop tsrp
bool "kopia: app == maintenance PASSWORD" '(.[0]|v("KOPIA_PASSWORD")) as $x | .[1]|v("KOPIA_PASSWORD") == $x' kopia vsmaint
bool "kopia: app == component PASSWORD" '(.[0]|v("KOPIA_PASSWORD")) as $x | .[1]|v("KOPIA_PASSWORD") == $x' kopia vscomp
bool "kopia: maintenance == component REPOSITORY" '(.[0]|v("KOPIA_REPOSITORY")) as $x | .[1]|v("KOPIA_REPOSITORY") == $x' vsmaint vscomp
bool "kopia: component REPOSITORY contains \${APP}" '.[0]|v("KOPIA_REPOSITORY") | contains("${APP}")' vscomp
bool "radarr: recyclarr == webhook key" '(.[0]|v("RADARR_API_KEY")) as $x | .[1]|v("RADARR__AUTH__APIKEY") == $x' recyc hook
bool "sonarr: recyclarr == webhook key" '(.[0]|v("SONARR_API_KEY")) as $x | .[1]|v("SONARR__AUTH__APIKEY") == $x' recyc hook
bool "github: flux webhook == konflate webhook" '(.[0]|v("token")) as $x | .[1]|v("KONFLATE_WEBHOOK_SECRET") == $x' fluxwh konfl
bool "mktxp: _mktxp.conf == mktxp.conf" '.[0]|v("_mktxp.conf") == v("mktxp.conf")' mktxp

echo "## Non-empty"
bool "grafana: ADMIN_USER non-empty" '.[0]|v("GF_SECURITY_ADMIN_USER") | (. // "") != ""' grafana
bool "grafana: ADMIN_PASSWORD non-empty" '.[0]|v("GF_SECURITY_ADMIN_PASSWORD") | (. // "") != ""' grafana
bool "rook-ceph: password non-empty" '.[0]|v("password") | (. // "") != ""' rook

echo "## Names"
ini='split("\n")[] | sub("^\\s+";"") | select(test("^\\[.+\\]$") or test("^[A-Za-z0-9_.-]+\\s*=")) | if startswith("[") then "section " + . else "key " + (split("=")[0] | sub("\\s+$";"")) end'
names "mktxp.conf (sections and keys, in order)" "v(\"mktxp.conf\") | $ini" mktxp
names "_mktxp.conf (sections and keys, in order)" "v(\"_mktxp.conf\") | $ini" mktxp
names "ghcr: registries" 'v(".dockerconfigjson") | fromjson | .auths | keys[]' ghcr
names "ghcr: fields per registry" 'v(".dockerconfigjson") | fromjson | .auths[] | keys | join(" ")' ghcr
