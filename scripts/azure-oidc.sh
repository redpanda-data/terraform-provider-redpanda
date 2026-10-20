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

# Stands up, checks, and tears down a local OIDC issuer that an Azure app
# registration trusts, so the byoc plugin can be exercised with workload
# identity federation and no client secret.
#
# Usage: azure-oidc.sh up|login|down
#
#   up     create the issuer site (a storage account static website serving
#          the discovery document and public key set) and add a federated
#          credential for it to the app registration
#   login  mint a token and exchange it for an Azure login in a throwaway az
#          config dir, proving Entra accepts the federation
#   down   delete the federated credential and the issuer resource group
#
# With AZURE_OIDC_IDENTITY=app, up and down change the app registration, so
# run them as an owner of it, for example with AZURE_CONFIG_DIR pointing at
# that owner's az login. With AZURE_OIDC_IDENTITY=managed, up creates a
# user-assigned managed identity in the issuer resource group, trusts the
# issuer from it, and grants it the subscription roles the byoc plugin needs;
# that takes only Contributor and User Access Administrator on the
# subscription. Its client id is recorded in AZURE_OIDC_DIR/client-id, and the
# byoc and vm runners present tokens for it instead of the app registration.
#
# Environment:
#   AZURE_OIDC_DIR         local state dir (default .azure-oidc)
#   AZURE_OIDC_IDENTITY    app (default) or managed, see above
#   AZURE_OIDC_APP_ID      app registration client id (default AZURE_CLIENT_ID)
#   AZURE_OIDC_SUBJECT     token subject the credential trusts (default tfrp-oidc-manual)
#   AZURE_OIDC_REGION      issuer site region (default eastus)
#   AZURE_SUBSCRIPTION_ID  subscription for the issuer site (or ARM_SUBSCRIPTION_ID)
#   AZURE_TENANT_ID        tenant for login (or ARM_TENANT_ID; default the az session's)

set -euo pipefail

cmd="${1:-}"
dir="${AZURE_OIDC_DIR:-.azure-oidc}"
identity="${AZURE_OIDC_IDENTITY:-app}"
app_id="${AZURE_OIDC_APP_ID:-${AZURE_CLIENT_ID:-${ARM_CLIENT_ID:-}}}"
subject="${AZURE_OIDC_SUBJECT:-tfrp-oidc-manual}"
region="${AZURE_OIDC_REGION:-eastus}"
subscription="${AZURE_SUBSCRIPTION_ID:-${ARM_SUBSCRIPTION_ID:-}}"
# Not tfrp-: the Azure sweepers delete every tfrp- resource group.
resource_group="rp-oidc-issuer"
credential_name="tfrp-local-oidc"
audience="api://AzureADTokenExchange"
identity_name="rp-oidc-workload"
# What the byoc plugin needs on the subscription: its resources, its state
# blobs, and role assignments for the agent's identity. The CI service
# principal holds User Access Administrator for the last, under a condition
# that stops it from granting Owner, User Access Administrator, or Role Based
# Access Control Administrator to anyone, so the managed identity gets a
# custom role holding only the role-assignment and role-definition actions,
# assigned under that same condition.
roles=("Contributor" "Storage Blob Data Contributor")
rbac_role="rp-oidc-byoc-rbac"
# Owner, User Access Administrator, and Role Based Access Control
# Administrator, by their built-in role definition IDs.
privileged_roles="{8e3af657-a8ff-443c-a75c-2fe8c4bcb635, 18d7d88d-d35e-4fb5-a5c3-7773c20a72d9, f58310d9-a9f6-439a-9e8d-f62e7b41a168}"
rbac_condition="((!(ActionMatches{'Microsoft.Authorization/roleAssignments/write'})) OR (@Request[Microsoft.Authorization/roleAssignments:RoleDefinitionId] ForAnyOfAllValues:GuidNotEquals $privileged_roles)) AND ((!(ActionMatches{'Microsoft.Authorization/roleAssignments/delete'})) OR (@Resource[Microsoft.Authorization/roleAssignments:RoleDefinitionId] ForAnyOfAllValues:GuidNotEquals $privileged_roles))"
key="$dir/signing-key.pem"
bin="$dir/bin/azureoidc"

die() { echo "azure-oidc: $*" >&2; exit 1; }

[[ "$identity" == app || "$identity" == managed ]] || die "AZURE_OIDC_IDENTITY must be app or managed"
[[ "$identity" == managed || -n "$app_id" ]] || die "set AZURE_OIDC_APP_ID or AZURE_CLIENT_ID to the app registration's client id"
[[ -n "$subscription" ]] || die "set AZURE_SUBSCRIPTION_ID or ARM_SUBSCRIPTION_ID"
[[ -x "$bin" ]] || die "$bin is missing; run task azure-oidc:key"

# Storage account names are global, 3-24 lowercase alphanumerics; derive one
# per subscription and app so reruns find the same site.
account="rpoidc$(printf '%s/%s' "$subscription" "$app_id" | shasum -a 256 | cut -c1-12)"

issuer_url() {
  az storage account show --subscription "$subscription" -g "$resource_group" -n "$account" \
    --query primaryEndpoints.web -o tsv | sed 's:/*$::'
}

up() {
  [[ -f "$key" ]] || die "$key is missing; run task azure-oidc:key"
  echo "azure-oidc: resource group $resource_group and storage account $account in $region"
  az group create --subscription "$subscription" -n "$resource_group" -l "$region" -o none
  if ! az storage account show --subscription "$subscription" -g "$resource_group" -n "$account" -o none 2>/dev/null; then
    az storage account create --subscription "$subscription" -g "$resource_group" -n "$account" -l "$region" \
      --kind StorageV2 --sku Standard_LRS --min-tls-version TLS1_2 --allow-blob-public-access false -o none
  fi
  local account_key
  account_key="$(az storage account keys list --subscription "$subscription" -g "$resource_group" -n "$account" --query '[0].value' -o tsv)"
  az storage blob service-properties update --account-name "$account" --account-key "$account_key" \
    --static-website --index-document index.html -o none

  local issuer site
  issuer="$(issuer_url)"
  [[ -n "$issuer" ]] || die "storage account $account has no static website endpoint"
  site="$dir/site"
  "$bin" publish -key "$key" -issuer "$issuer" -out "$site"
  for path in .well-known/openid-configuration jwks.json; do
    az storage blob upload --account-name "$account" --account-key "$account_key" \
      --container-name "\$web" --name "$path" --file "$site/$path" \
      --content-type application/json --overwrite -o none
  done
  printf '%s' "$issuer" > "$dir/issuer"

  # A newly enabled static website answers 404 for a short while.
  local served=""
  for _ in $(seq 1 24); do
    served="$(curl -fsS "$issuer/.well-known/openid-configuration" 2>/dev/null | jq -r .issuer || true)"
    [[ "$served" == "$issuer" ]] && break
    sleep 5
  done
  [[ "$served" == "$issuer" ]] || die "discovery document at $issuer serves issuer '$served'"

  trust "$issuer"
}

# trust adds the federated credential for the issuer to the principal.
trust() {
  local issuer="$1"
  if [[ "$identity" == managed ]]; then
    trust_managed "$issuer"
    return
  fi
  local params
  params="$(jq -n --arg n "$credential_name" --arg i "$issuer" --arg s "$subject" --arg a "$audience" \
    '{name: $n, issuer: $i, subject: $s, audiences: [$a], description: "Local OIDC issuer for byoc plugin workload identity tests"}')"
  if az ad app federated-credential show --id "$app_id" --federated-credential-id "$credential_name" -o none 2>/dev/null; then
    az ad app federated-credential update --id "$app_id" --federated-credential-id "$credential_name" --parameters "$params" -o none
  else
    az ad app federated-credential create --id "$app_id" --parameters "$params" -o none
  fi
  echo "azure-oidc: issuer $issuer trusted by app $app_id for subject $subject"
}

# trust_managed creates the managed identity, trusts the issuer from it, and
# grants it the subscription roles. Every step is an upsert, so a rerun after
# a partial failure completes it.
trust_managed() {
  local issuer="$1" client_id principal_id
  az identity create --subscription "$subscription" -g "$resource_group" -n "$identity_name" -l "$region" -o none
  client_id="$(az identity show --subscription "$subscription" -g "$resource_group" -n "$identity_name" --query clientId -o tsv)"
  principal_id="$(az identity show --subscription "$subscription" -g "$resource_group" -n "$identity_name" --query principalId -o tsv)"
  local verb=create
  if az identity federated-credential show --subscription "$subscription" -g "$resource_group" \
    --identity-name "$identity_name" -n "$credential_name" -o none 2>/dev/null; then
    verb=update
  fi
  az identity federated-credential "$verb" --subscription "$subscription" -g "$resource_group" \
    --identity-name "$identity_name" -n "$credential_name" \
    --issuer "$issuer" --subject "$subject" --audiences "$audience" -o none
  # Recorded before the roles, so login can prove the federation even when a
  # role grant is refused.
  printf '%s' "$client_id" > "$dir/client-id"
  local role
  for role in "${roles[@]}"; do
    assign_role "$principal_id" "$role"
  done
  ensure_rbac_role
  assign_role "$principal_id" "$rbac_role" "$rbac_condition"
  rm -f "$dir/role.err"
  echo "azure-oidc: issuer $issuer trusted by managed identity $identity_name ($client_id) for subject $subject"
}

# assign_role grants role to the principal at the subscription scope, with an
# ABAC condition when one is given. Rerunning with the same role is a no-op.
assign_role() {
  local principal_id="$1" role="$2" condition="${3:-}"
  local extra=()
  [[ -n "$condition" ]] && extra=(--condition "$condition" --condition-version 2.0)
  # A new principal or role takes a moment to reach the authorization service.
  for attempt in 1 2 3 4 5 6; do
    if az role assignment create --assignee-object-id "$principal_id" --assignee-principal-type ServicePrincipal \
      --role "$role" --scope "/subscriptions/$subscription" ${extra[@]+"${extra[@]}"} -o none 2>"$dir/role.err"; then
      return
    fi
    # An authorization refusal does not clear with time.
    grep -q AuthorizationFailed "$dir/role.err" && die "assigning $role: $(head -c 400 "$dir/role.err")"
    [[ "$attempt" -lt 6 ]] || die "assigning $role: $(head -c 400 "$dir/role.err")"
    sleep 10
  done
}

# ensure_rbac_role creates or updates the custom role the agent stage needs:
# it defines a role for the agent's identity and assigns it.
ensure_rbac_role() {
  local definition verb=create
  definition="$(jq -n --arg n "$rbac_role" --arg s "/subscriptions/$subscription" '{
    Name: $n,
    Description: "Role assignments and role definitions for the byoc agent, for the local OIDC managed identity",
    Actions: [
      "Microsoft.Authorization/roleAssignments/read",
      "Microsoft.Authorization/roleAssignments/write",
      "Microsoft.Authorization/roleAssignments/delete",
      "Microsoft.Authorization/roleDefinitions/read",
      "Microsoft.Authorization/roleDefinitions/write",
      "Microsoft.Authorization/roleDefinitions/delete"
    ],
    AssignableScopes: [$s]
  }')"
  if [[ -n "$(az role definition list --subscription "$subscription" --name "$rbac_role" --query '[0].name' -o tsv)" ]]; then
    verb=update
  fi
  az role definition "$verb" --subscription "$subscription" --role-definition "$definition" -o none
}

# client_id prints the client id tokens are exchanged for.
client_id() {
  if [[ "$identity" == managed ]]; then
    [[ -f "$dir/client-id" ]] || die "no managed identity recorded; run task azure-oidc:up with AZURE_OIDC_IDENTITY=managed"
    cat "$dir/client-id"
  else
    printf '%s' "$app_id"
  fi
}

login() {
  [[ -f "$dir/issuer" ]] || die "no issuer recorded; run task azure-oidc:up"
  local tenant token scratch
  tenant="${AZURE_TENANT_ID:-${ARM_TENANT_ID:-$(az account show --query tenantId -o tsv)}}"
  token="$dir/token"
  "$bin" mint -key "$key" -issuer "$(cat "$dir/issuer")" -subject "$subject" -lifetime 10m -out "$token"
  scratch="$(mktemp -d)"
  trap 'rm -rf "$scratch"' RETURN
  # A new federated credential can take a minute to reach every Entra node.
  for attempt in 1 2 3 4 5 6; do
    if AZURE_CONFIG_DIR="$scratch" az login --service-principal -u "$(client_id)" -t "$tenant" \
      --federated-token "$(cat "$token")" --allow-no-subscriptions -o none 2>"$scratch/err"; then
      AZURE_CONFIG_DIR="$scratch" az account show --query '{tenant: tenantId, user: user.name}' -o json
      echo "azure-oidc: Entra accepted the federated token"
      return 0
    fi
    echo "azure-oidc: login attempt $attempt failed: $(head -c 400 "$scratch/err")" >&2
    sleep 20
  done
  die "Entra rejected the federated token"
}

down() {
  if [[ "$identity" == managed ]]; then
    local principal_id
    principal_id="$(az identity show --subscription "$subscription" -g "$resource_group" -n "$identity_name" --query principalId -o tsv 2>/dev/null || true)"
    if [[ -n "$principal_id" ]]; then
      # The resource group takes the identity with it; its subscription-scope
      # role assignments would otherwise linger as orphans.
      az role assignment delete --assignee "$principal_id" --scope "/subscriptions/$subscription" -o none
      echo "azure-oidc: removed the role assignments of managed identity $identity_name"
    fi
    if [[ -n "$(az role definition list --subscription "$subscription" --name "$rbac_role" --query '[0].name' -o tsv)" ]]; then
      az role definition delete --subscription "$subscription" --name "$rbac_role"
      echo "azure-oidc: deleted custom role $rbac_role"
    fi
    rm -f "$dir/client-id"
  elif az ad app federated-credential show --id "$app_id" --federated-credential-id "$credential_name" -o none 2>/dev/null; then
    az ad app federated-credential delete --id "$app_id" --federated-credential-id "$credential_name"
    echo "azure-oidc: deleted federated credential $credential_name"
  fi
  if [[ "$(az group exists --subscription "$subscription" -n "$resource_group")" == "true" ]]; then
    az group delete --subscription "$subscription" -n "$resource_group" --yes
    echo "azure-oidc: deleted resource group $resource_group"
  fi
  rm -f "$dir/issuer" "$dir/token"
}

case "$cmd" in
  up) up ;;
  login) login ;;
  down) down ;;
  *) die "usage: azure-oidc.sh up|login|down" ;;
esac
