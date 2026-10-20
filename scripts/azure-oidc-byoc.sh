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
# Usage: azure-oidc-byoc.sh apply|plan|destroy local|released token-file|relative-path|arm-token|customer|actions|cli-session|secret
#
#   plan runs with -detailed-exitcode, so exit 0 is an empty plan and exit 2
#   a drift, against the cluster from the last apply.
#
#   local      the provider built from this checkout (dev_overrides)
#   released   the provider from the registry (REDPANDA_VERSION pins it)
#
#   token-file ARM_USE_OIDC with AZURE_FEDERATED_TOKEN_FILE
#   relative-path ARM_USE_OIDC with ARM_OIDC_TOKEN_FILE_PATH naming the token
#              relative to the Terraform working directory, as azurerm
#              reads it
#   arm-token  ARM_USE_OIDC with the raw token in ARM_OIDC_TOKEN
#   customer   ARM_USE_OIDC after az login --federated-token, with a stub of
#              the GitHub Actions ID-token endpoint and no token file, the way
#              azure/login leaves a GitHub Actions job
#   actions    ARM_USE_OIDC with only the GitHub Actions ID-token endpoint
#              stub: no token file and no az login, the way a job with the
#              id-token: write permission and no azure/login step runs
#   cli-session ARM_USE_OIDC with no token, no endpoint, and the ambient az
#              login kept, the way a laptop with az login and ARM_USE_OIDC
#              runs; needs no issuer
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
# REDPANDA_VERSION, AZURE_OIDC_PROVIDER_BINARY (a prebuilt local provider, so
# no Go toolchain is needed), AZURE_OIDC_TOKEN_LIFETIME (the lifetime of
# every token this script mints, for the file modes and the GitHub Actions
# stub alike; default 5m, which is what GitHub issues, so a static token file
# expires under an apply the way a customer's does; set 8h for a run that is
# not about lifetimes), and AZURE_OIDC_WORK (the Terraform working directory, default
# AZURE_OIDC_DIR/byoc; a different one per run lets runs overlap). When
# azure-oidc.sh up created a managed identity, AZURE_OIDC_DIR/client-id
# replaces AZURE_CLIENT_ID in the OIDC modes.

set -euo pipefail

action="${1:-}"
provider="${2:-}"
auth="${3:-}"
case "$action/$provider/$auth" in
  apply/local/* | apply/released/* | plan/local/* | plan/released/* | destroy/local/* | destroy/released/*) ;;
  *) echo "usage: azure-oidc-byoc.sh apply|plan|destroy local|released token-file|relative-path|arm-token|customer|actions|cli-session|secret" >&2; exit 2 ;;
esac

repo="$(pwd)"
dir="$repo/${AZURE_OIDC_DIR:-.azure-oidc}"
bin="$dir/bin/azureoidc"
key="$dir/signing-key.pem"
subject="${AZURE_OIDC_SUBJECT:-tfrp-oidc-manual}"
region="${AZURE_REGION:-eastus}"
# Not tfrp-: the CI sweepers delete every tfrp- cluster, network, and group.
name="${AZURE_OIDC_NAME:-rp-oidc-$(whoami | tr -cd 'a-z0-9' | cut -c1-12)}"
work="${AZURE_OIDC_WORK:-$dir/byoc}"
# Terraform runs from $work and reads TF_CLI_CONFIG_FILE relative to it.
[[ "$work" == /* ]] || work="$repo/$work"
logs="$dir/logs"
token_lifetime="${AZURE_OIDC_TOKEN_LIFETIME:-5m}"

die() { echo "azure-oidc-byoc: $*" >&2; exit 1; }

[[ -n "${REDPANDA_CLIENT_ID:-}" && -n "${REDPANDA_CLIENT_SECRET:-}" ]] || die "set REDPANDA_CLIENT_ID and REDPANDA_CLIENT_SECRET"
[[ -n "${AZURE_CLIENT_ID:-}" && -n "${AZURE_TENANT_ID:-}" ]] || die "set AZURE_CLIENT_ID and AZURE_TENANT_ID"
export ARM_SUBSCRIPTION_ID="${ARM_SUBSCRIPTION_ID:-${AZURE_SUBSCRIPTION_ID:-}}"
[[ -n "$ARM_SUBSCRIPTION_ID" ]] || die "set ARM_SUBSCRIPTION_ID or AZURE_SUBSCRIPTION_ID"
# A managed identity from azure-oidc.sh up stands in for the app registration
# in every mode that presents a token; the provider reads either variable.
if [[ -f "$dir/client-id" && "$auth" != cli-session && "$auth" != secret ]]; then
  AZURE_CLIENT_ID="$(cat "$dir/client-id")"
  export AZURE_CLIENT_ID ARM_CLIENT_ID="$AZURE_CLIENT_ID"
fi

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

if [[ "$auth" == cli-session ]]; then
  unset AZURE_CLIENT_SECRET ARM_CLIENT_SECRET ARM_CLIENT_CERTIFICATE ARM_CLIENT_CERTIFICATE_PATH \
    AZURE_CLIENT_CERTIFICATE_PATH AZURE_FEDERATED_TOKEN_FILE ARM_OIDC_TOKEN ARM_OIDC_TOKEN_FILE_PATH \
    ARM_USE_CLI ARM_USE_MSI ARM_USE_AKS_WORKLOAD_IDENTITY \
    ACTIONS_ID_TOKEN_REQUEST_URL ACTIONS_ID_TOKEN_REQUEST_TOKEN ARM_OIDC_REQUEST_URL ARM_OIDC_REQUEST_TOKEN
  export ARM_USE_OIDC=true
  az account show -o none || die "cli-session needs an ambient az login"
elif [[ "$auth" != secret ]]; then
  # Credentials only: AZURE_TOKEN_CREDENTIALS passes through so a run can
  # override what the provider would set.
  unset AZURE_CLIENT_SECRET ARM_CLIENT_SECRET ARM_CLIENT_CERTIFICATE ARM_CLIENT_CERTIFICATE_PATH \
    AZURE_CLIENT_CERTIFICATE_PATH AZURE_FEDERATED_TOKEN_FILE ARM_OIDC_TOKEN ARM_OIDC_TOKEN_FILE_PATH \
    ARM_USE_CLI ARM_USE_MSI ARM_USE_AKS_WORKLOAD_IDENTITY
  export AZURE_CONFIG_DIR="$scratch/az"
  export ARM_USE_OIDC=true
  token="$scratch/token"
  [[ -f "$dir/issuer" ]] || die "no issuer recorded; run task azure-oidc:up"
  "$bin" mint -key "$key" -issuer "$(cat "$dir/issuer")" -subject "$subject" -lifetime "$token_lifetime" -out "$token"
  case "$auth" in
    token-file)
      export AZURE_FEDERATED_TOKEN_FILE="$token"
      ;;
    relative-path)
      # Terraform runs from $work below, so this names $work/oidc-token.
      cp "$token" "$work/oidc-token"
      export ARM_OIDC_TOKEN_FILE_PATH=./oidc-token
      ;;
    arm-token)
      ARM_OIDC_TOKEN="$(cat "$token")"
      export ARM_OIDC_TOKEN
      ;;
    customer | actions)
      if [[ "$auth" == customer ]]; then
        az login --service-principal -u "$AZURE_CLIENT_ID" -t "$AZURE_TENANT_ID" \
          --federated-token "$(cat "$token")" --allow-no-subscriptions -o none
        az account set --subscription "$ARM_SUBSCRIPTION_ID"
      fi
      request_token="$(openssl rand -hex 16)"
      "$bin" serve-actions -key "$key" -issuer "$(cat "$dir/issuer")" -subject "$subject" \
        -lifetime "$token_lifetime" -request-token "$request_token" -url-file "$scratch/actions-url" >/dev/null &
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
echo "azure-oidc-byoc: $action with the $provider provider, $auth auth, token lifetime $token_lifetime, cluster $name in $region, working directory $work"
echo "azure-oidc-byoc: log $log"
cd "$work"
if [[ "$provider" == released ]]; then
  terraform init -upgrade -input=false >>"$log" 2>&1
fi
# Only the cluster and what it needs: the example's data-plane resources need
# the cluster API, which is not reachable from every workstation.
target=()
[[ "$action" == apply || "$action" == plan ]] && target=(-target=redpanda_cluster.test)
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
if [[ "$action" == plan ]]; then
  terraform plan -detailed-exitcode -input=false ${target[@]+"${target[@]}"} 2>&1 | tee -a "$log"
  exit "${PIPESTATUS[0]}"
fi
terraform "$action" -auto-approve -input=false ${target[@]+"${target[@]}"} 2>&1 | tee -a "$log"
