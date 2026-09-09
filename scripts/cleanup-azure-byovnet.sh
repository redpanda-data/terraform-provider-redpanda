#!/usr/bin/env bash
# Deletes the resource groups the Azure BYOVNet acceptance lane leaves behind.
# The producer stack's Terraform state lives in the test container, so a later
# Buildkite step cannot `terraform destroy` it; deleting every resource group
# under the run prefix removes the same resources. Key vaults inside carry
# purge protection, so they stay soft-deleted for 90 days and cannot be
# purged here; their names are per-run, which is what avoids collisions.
#
# Usage: cleanup-azure-byovnet.sh [--dry-run] [--auto-approve] [prefix]
# Auth: an existing `az login`, or ARM_*/AZURE_* service-principal variables.
set -euo pipefail

dry_run=false
auto_approve=false
prefix="tfrp-"
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=true ;;
    --auto-approve) auto_approve=true ;;
    *) prefix="$arg" ;;
  esac
done

if ! command -v az >/dev/null 2>&1; then
  echo "cleanup-azure-byovnet: az CLI not found" >&2
  exit 1
fi

client_id="${ARM_CLIENT_ID:-${AZURE_CLIENT_ID:-}}"
client_secret="${ARM_CLIENT_SECRET:-${AZURE_CLIENT_SECRET:-}}"
tenant_id="${ARM_TENANT_ID:-${AZURE_TENANT_ID:-}}"
subscription_id="${ARM_SUBSCRIPTION_ID:-${AZURE_SUBSCRIPTION_ID:-}}"
if ! az account show --output none 2>/dev/null; then
  if [[ -z "$client_id" || -z "$client_secret" || -z "$tenant_id" ]]; then
    echo "cleanup-azure-byovnet: not logged in and no service-principal credentials in the environment" >&2
    exit 1
  fi
  az login --service-principal -u "$client_id" -p "$client_secret" --tenant "$tenant_id" --output none
fi
if [[ -n "$subscription_id" ]]; then
  az account set --subscription "$subscription_id"
fi

groups=()
while IFS= read -r g; do [[ -n "$g" ]] && groups+=("$g"); done < <(az group list --query "[?starts_with(name, '$prefix')].name" --output tsv)
if (( ${#groups[@]} == 0 )); then
  echo "cleanup-azure-byovnet: no resource groups with prefix '$prefix'"
  exit 0
fi

echo "cleanup-azure-byovnet: resource groups with prefix '$prefix':"
printf '  %s\n' "${groups[@]}"
if $dry_run; then
  echo "cleanup-azure-byovnet: dry run, nothing deleted"
  exit 0
fi
if ! $auto_approve; then
  read -r -p "Delete these ${#groups[@]} resource groups? [y/N] " answer
  [[ "$answer" == [yY] ]] || { echo "aborted"; exit 1; }
fi

for g in "${groups[@]}"; do
  az group delete --name "$g" --yes --no-wait --output none
  echo "  queued delete of $g"
done

for _ in $(seq 1 60); do
  left="$(az group list --query "length([?starts_with(name, '$prefix')])" --output tsv)"
  if [[ "$left" == "0" ]]; then
    echo "cleanup-azure-byovnet: all resource groups deleted"
    exit 0
  fi
  sleep 15
done
echo "cleanup-azure-byovnet: $left resource group(s) still deleting after 15 minutes; Azure finishes them asynchronously" >&2
