#!/usr/bin/env bash
# Copyright 2026 Redpanda Data, Inc.
#
#    Licensed under the Apache License, Version 2.0 (the "License");
#    you may not use this file except in compliance with the License.
#    You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.

# Applies or destroys an Azure BYOC cluster (examples/byoc/azure) with the
# provider authenticating to Azure through the local OIDC issuer from
# azure-oidc.sh, so the byoc plugin runs with no client secret.
#
# Usage: azure-oidc-byoc.sh apply|destroy local|released token-file|arm-token|customer|secret
#
#   local      the provider built from this checkout (dev_overrides)
#   released   the provider from the registry (REDPANDA_VERSION pins it)
#
#   token-file ARM_USE_OIDC with AZURE_FEDERATED_TOKEN_FILE
#   arm-token  ARM_USE_OIDC with the raw token in ARM_OIDC_TOKEN
#   customer   ARM_USE_OIDC after az login --federated-token, with a stub of
#              the GitHub Actions ID-token endpoint and no token file, the way
#              azure/login leaves a GitHub Actions job
#   secret     the ambient client secret, for cleaning up after a failed run
#
# Every OIDC mode runs with the client secret and certificate unset and an
# empty AZURE_CONFIG_DIR, so no ambient az login can stand in for the path
# under test. State and logs stay in AZURE_OIDC_DIR/byoc across runs.
#
# Environment: REDPANDA_CLIENT_ID, REDPANDA_CLIENT_SECRET,
# REDPANDA_CLOUD_ENVIRONMENT, AZURE_CLIENT_ID, AZURE_TENANT_ID,
# ARM_SUBSCRIPTION_ID (or AZURE_SUBSCRIPTION_ID), and optionally
# AZURE_OIDC_DIR, AZURE_OIDC_SUBJECT, AZURE_OIDC_NAME, AZURE_REGION,
# REDPANDA_VERSION, AZURE_OIDC_TOKEN_FILE (a pre-minted token, so no signing
# key is needed), and AZURE_OIDC_PROVIDER_BINARY (a prebuilt local provider,
# so no Go toolchain is needed).

set -euo pipefail

action="${1:-}"
provider="${2:-}"
auth="${3:-}"
case "$action/$provider/$auth" in
  apply/local/* | apply/released/* | destroy/local/* | destroy/released/*) ;;
  *) echo "usage: azure-oidc-byoc.sh apply|destroy local|released token-file|arm-token|customer|secret" >&2; exit 2 ;;
esac

repo="$(pwd)"
dir="$repo/${AZURE_OIDC_DIR:-.azure-oidc}"
bin="$dir/bin/azureoidc"
key="$dir/signing-key.pem"
subject="${AZURE_OIDC_SUBJECT:-tfrp-oidc-manual}"
region="${AZURE_REGION:-eastus}"
# Not tfrp-: the CI sweepers delete every tfrp- cluster, network, and group.
name="${AZURE_OIDC_NAME:-rp-oidc-$(whoami | tr -cd 'a-z0-9' | cut -c1-12)}"
work="$dir/byoc"
logs="$dir/logs"

die() { echo "azure-oidc-byoc: $*" >&2; exit 1; }

[[ -n "${REDPANDA_CLIENT_ID:-}" && -n "${REDPANDA_CLIENT_SECRET:-}" ]] || die "set REDPANDA_CLIENT_ID and REDPANDA_CLIENT_SECRET"
[[ -n "${AZURE_CLIENT_ID:-}" && -n "${AZURE_TENANT_ID:-}" ]] || die "set AZURE_CLIENT_ID and AZURE_TENANT_ID"
export ARM_SUBSCRIPTION_ID="${ARM_SUBSCRIPTION_ID:-${AZURE_SUBSCRIPTION_ID:-}}"
[[ -n "$ARM_SUBSCRIPTION_ID" ]] || die "set ARM_SUBSCRIPTION_ID or AZURE_SUBSCRIPTION_ID"

mkdir -p "$work" "$logs"
cp "$repo"/examples/byoc/azure/*.tf "$work"/

if [[ "$provider" == local ]]; then
  mkdir -p "$work/provider"
  if [[ -n "${AZURE_OIDC_PROVIDER_BINARY:-}" ]]; then
    cp "$AZURE_OIDC_PROVIDER_BINARY" "$work/provider/terraform-provider-redpanda"
  else
    (cd "$repo" && go build -o "$work/provider/terraform-provider-redpanda" .)
  fi
  cat > "$work/.terraformrc" <<EOF
provider_installation {
  dev_overrides {
    "redpanda-data/redpanda" = "$work/provider"
  }
  direct {}
}
EOF
else
  printf 'provider_installation {\n  direct {}\n}\n' > "$work/.terraformrc"
  if [[ -n "${REDPANDA_VERSION:-}" ]]; then
    cat > "$work/versions.tf" <<EOF
terraform {
  required_providers {
    redpanda = {
      source  = "redpanda-data/redpanda"
      version = "= $REDPANDA_VERSION"
    }
  }
}
EOF
  fi
fi
export TF_CLI_CONFIG_FILE="$work/.terraformrc"

scratch="$(mktemp -d)"
stub_pid=""
cleanup() {
  [[ -n "$stub_pid" ]] && kill "$stub_pid" 2>/dev/null || true
  rm -rf "$scratch"
}
trap cleanup EXIT

# The SDK's credential attempts are the point of these runs; a caller's own
# setting wins.
export AZURE_SDK_GO_LOGGING="${AZURE_SDK_GO_LOGGING:-all}"

if [[ "$auth" != secret ]]; then
  # Credentials only: AZURE_TOKEN_CREDENTIALS passes through so a run can
  # override what the provider would set.
  unset AZURE_CLIENT_SECRET ARM_CLIENT_SECRET ARM_CLIENT_CERTIFICATE ARM_CLIENT_CERTIFICATE_PATH \
    AZURE_CLIENT_CERTIFICATE_PATH AZURE_FEDERATED_TOKEN_FILE ARM_OIDC_TOKEN ARM_OIDC_TOKEN_FILE_PATH \
    ARM_USE_CLI ARM_USE_MSI ARM_USE_AKS_WORKLOAD_IDENTITY
  export AZURE_CONFIG_DIR="$scratch/az"
  export ARM_USE_OIDC=true
  token="$scratch/token"
  if [[ -n "${AZURE_OIDC_TOKEN_FILE:-}" ]]; then
    # A token minted elsewhere, so this host never holds the signing key.
    cp "$AZURE_OIDC_TOKEN_FILE" "$token"
  else
    [[ -f "$dir/issuer" ]] || die "no issuer recorded; run task azure-oidc:up"
    # Entra accepts long-lived assertions, so one token outlasts a BYOC apply.
    "$bin" mint -key "$key" -issuer "$(cat "$dir/issuer")" -subject "$subject" -lifetime 8h -out "$token"
  fi
  case "$auth" in
    token-file)
      export AZURE_FEDERATED_TOKEN_FILE="$token"
      ;;
    arm-token)
      ARM_OIDC_TOKEN="$(cat "$token")"
      export ARM_OIDC_TOKEN
      ;;
    customer)
      az login --service-principal -u "$AZURE_CLIENT_ID" -t "$AZURE_TENANT_ID" \
        --federated-token "$(cat "$token")" --allow-no-subscriptions -o none
      az account set --subscription "$ARM_SUBSCRIPTION_ID"
      request_token="$(openssl rand -hex 16)"
      if [[ -n "${AZURE_OIDC_TOKEN_FILE:-}" ]]; then
        "$bin" serve-actions -token-file "$token" -request-token "$request_token" \
          -url-file "$scratch/actions-url" >/dev/null &
      else
        "$bin" serve-actions -key "$key" -issuer "$(cat "$dir/issuer")" -subject "$subject" \
          -lifetime 1h -request-token "$request_token" -url-file "$scratch/actions-url" >/dev/null &
      fi
      stub_pid=$!
      for _ in $(seq 1 50); do [[ -s "$scratch/actions-url" ]] && break; sleep 0.1; done
      [[ -s "$scratch/actions-url" ]] || die "GitHub Actions token stub did not start"
      export ACTIONS_ID_TOKEN_REQUEST_URL
      ACTIONS_ID_TOKEN_REQUEST_URL="$(cat "$scratch/actions-url")"
      export ACTIONS_ID_TOKEN_REQUEST_TOKEN="$request_token"
      ;;
    *) die "unknown auth mode $auth" ;;
  esac
fi

log="$logs/$(date +%Y%m%d-%H%M%S)-$action-$provider-$auth.log"
echo "azure-oidc-byoc: $action with the $provider provider, $auth auth, cluster $name in $region"
echo "azure-oidc-byoc: log $log"
cd "$work"
if [[ "$provider" == released ]]; then
  terraform init -upgrade -input=false >>"$log" 2>&1
fi
# Only the cluster and what it needs: the example's data-plane resources need
# the cluster API, which is not reachable from every workstation.
target=()
[[ "$action" == apply ]] && target=(-target=redpanda_cluster.test)
# A tfvars file rather than -var: the example declares zones without a type,
# so a -var value arrives as a string.
cat > run.auto.tfvars <<EOF
resource_group_name    = "$name"
network_name           = "$name"
cluster_name           = "$name"
region                 = "$region"
zones                  = ["$region-az1", "$region-az2", "$region-az3"]
cluster_allow_deletion = true
EOF
terraform "$action" -auto-approve -input=false ${target[@]+"${target[@]}"} 2>&1 | tee -a "$log"
