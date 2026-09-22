#!/usr/bin/env bash
# Waits until the caller can list blobs in the byovnet module's management
# container with Entra ID auth. The module grants the applying identity
# Storage Blob Data Contributor on that account, but Azure RBAC assignments
# take minutes to propagate; starting the BYOC agent bootstrap before then
# fails its Terraform state backend with AuthorizationPermissionMismatch.
#
# Usage: azure-storage-access-wait.sh <storage-account> <container> [timeout-seconds]
# Auth: an existing `az login`, or ARM_*/AZURE_* service-principal variables.
set -euo pipefail

account="${1:?storage account}"
container="${2:?container}"
timeout="${3:-600}"

client_id="${ARM_CLIENT_ID:-${AZURE_CLIENT_ID:-}}"
client_secret="${ARM_CLIENT_SECRET:-${AZURE_CLIENT_SECRET:-}}"
tenant_id="${ARM_TENANT_ID:-${AZURE_TENANT_ID:-}}"
if ! az account show --output none 2>/dev/null; then
  if [[ -z "$client_id" || -z "$client_secret" || -z "$tenant_id" ]]; then
    echo "azure-storage-access-wait: not logged in and no service-principal credentials in the environment" >&2
    exit 1
  fi
  az login --service-principal -u "$client_id" -p "$client_secret" --tenant "$tenant_id" --output none
fi

deadline=$((SECONDS + timeout))
until az storage blob list --account-name "$account" --container-name "$container" --auth-mode login --num-results 1 --output none 2>/dev/null; do
  if (( SECONDS >= deadline )); then
    echo "azure-storage-access-wait: still no blob access to $account/$container after ${timeout}s" >&2
    exit 1
  fi
  echo "azure-storage-access-wait: waiting for blob access to $account/$container (RBAC propagation)..."
  sleep 15
done
echo "azure-storage-access-wait: blob access to $account/$container confirmed"
