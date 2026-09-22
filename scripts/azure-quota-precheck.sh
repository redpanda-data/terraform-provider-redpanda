#!/usr/bin/env bash
# Fails fast when the subscription cannot fit a tier-1 Azure BYOC cluster in
# the region, so the lane does not spend an hour provisioning AKS only to have
# a node pool stick in Updating on a vCPU quota wall. tier-1-azure-v3-x86 runs
# three Standard_D2d_v5 brokers (DDv5) and two Standard_D2ads_v5 utility nodes
# (DADSv5); the Redpanda Connect pool draws on DDv5 too, so the DDv5 need
# carries an allowance for it.
#
# Usage: azure-quota-precheck.sh <region> [ddv5_need] [dadsv5_need] [public_ip_need]
# Auth: an existing `az login`, or ARM_CLIENT_ID/ARM_CLIENT_SECRET/
# ARM_TENANT_ID/ARM_SUBSCRIPTION_ID (AZURE_* accepted as a fallback).
set -euo pipefail

region="${1:?region}"
ddv5_need="${2:-12}"
dadsv5_need="${3:-4}"
public_ip_need="${4:-4}"

if ! command -v az >/dev/null 2>&1; then
  echo "azure-quota-precheck: az CLI not found; install it (brew install azure-cli, or https://aka.ms/InstallAzureCLIDeb)" >&2
  exit 1
fi

client_id="${ARM_CLIENT_ID:-${AZURE_CLIENT_ID:-}}"
client_secret="${ARM_CLIENT_SECRET:-${AZURE_CLIENT_SECRET:-}}"
tenant_id="${ARM_TENANT_ID:-${AZURE_TENANT_ID:-}}"
subscription_id="${ARM_SUBSCRIPTION_ID:-${AZURE_SUBSCRIPTION_ID:-}}"

if ! az account show --output none 2>/dev/null; then
  if [[ -z "$client_id" || -z "$client_secret" || -z "$tenant_id" ]]; then
    echo "azure-quota-precheck: not logged in and no service-principal credentials in the environment" >&2
    exit 1
  fi
  az login --service-principal -u "$client_id" -p "$client_secret" --tenant "$tenant_id" --output none
fi
if [[ -n "$subscription_id" ]]; then
  az account set --subscription "$subscription_id"
fi

usage_json="$(az vm list-usage --location "$region" --output json)"

check_family() {
  local family="$1" need="$2"
  local limit current free
  limit="$(jq -r --arg f "$family" '.[] | select(.name.value == $f) | .limit' <<<"$usage_json")"
  current="$(jq -r --arg f "$family" '.[] | select(.name.value == $f) | .currentValue' <<<"$usage_json")"
  if [[ -z "$limit" || "$limit" == "null" ]]; then
    echo "azure-quota-precheck: family $family not reported in $region" >&2
    exit 1
  fi
  free=$((limit - current))
  echo "azure-quota-precheck: $family in $region: $current/$limit used, $free free, need $need"
  if (( free < need )); then
    echo "azure-quota-precheck: insufficient $family vCPU quota in $region (raise it with: az quota update --resource-name $family --scope /subscriptions/<id>/providers/Microsoft.Compute/locations/$region --limit-object value=<N> --resource-type dedicated)" >&2
    exit 1
  fi
}

check_family standardDDv5Family "$ddv5_need"
check_family standardDADSv5Family "$dadsv5_need"

# The agent's cluster-network stack allocates two /31 public IP prefixes per
# cluster (NAT and load balancer), four Standard IPv4 addresses against a
# per-region subscription cap that the vCPU check does not see.
network_usage_json="$(az network list-usages --location "$region" --output json)"
check_network() {
  local counter="$1" need="$2"
  local limit current free
  limit="$(jq -r --arg c "$counter" '.[] | select(.name.value == $c) | .limit' <<<"$network_usage_json")"
  current="$(jq -r --arg c "$counter" '.[] | select(.name.value == $c) | .currentValue' <<<"$network_usage_json")"
  if [[ -z "$limit" || "$limit" == "null" ]]; then
    echo "azure-quota-precheck: counter $counter not reported in $region" >&2
    exit 1
  fi
  free=$((limit - current))
  echo "azure-quota-precheck: $counter in $region: $current/$limit used, $free free, need $need"
  if (( free < need )); then
    echo "azure-quota-precheck: insufficient $counter in $region (raise it with: az quota update --resource-name $counter --scope /subscriptions/<id>/providers/Microsoft.Network/locations/$region --limit-object value=<N> --resource-type $counter)" >&2
    exit 1
  fi
}

check_network IPv4StandardSkuPublicIpAddresses "${public_ip_need:-4}"
