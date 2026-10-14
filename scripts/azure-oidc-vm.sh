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

# Runs azure-oidc-byoc.sh on an Azure VM. Its IMDS answers a token request for
# the app's client ID with "Identity not found", as a GitHub-hosted runner's
# does, which is where DefaultAzureCredential stops before the Azure CLI. The signing key stays on this machine: each run copies over a token
# minted here, and the VM serves that token as the GitHub Actions endpoint.
#
# Usage: azure-oidc-vm.sh up|check|run ACTION PROVIDER AUTH|wait|down
#
#   up     create the VM (SSH open to this machine's public IP only)
#   check  confirm the VM's IMDS answers "Identity not found"
#   run    build, copy, and start azure-oidc-byoc.sh on the VM, then follow it
#   wait   follow a started run until it finishes
#   down   delete the VM; refuses while its Terraform state still holds
#          resources unless FORCE=1
#
# Environment: as for azure-oidc-byoc.sh, plus AZURE_OIDC_VM_ENV, extra
# NAME=value pairs exported on the VM for a run (for example
# "AZURE_TOKEN_CREDENTIALS=prod").

set -euo pipefail

cmd="${1:-}"
dir="${AZURE_OIDC_DIR:-.azure-oidc}"
vmdir="$dir/vm"
subscription="${AZURE_SUBSCRIPTION_ID:-${ARM_SUBSCRIPTION_ID:-}}"
region="${AZURE_REGION:-eastus}"
# Not tfrp-: the Azure sweepers delete every tfrp- resource group.
resource_group="rp-oidc-vm"
vm="rp-oidc-runner"
user="azureuser"
ssh_key="$vmdir/id_ed25519"

die() { echo "azure-oidc-vm: $*" >&2; exit 1; }
[[ -n "$subscription" ]] || die "set AZURE_SUBSCRIPTION_ID or ARM_SUBSCRIPTION_ID"

remote() {
  [[ -f "$vmdir/host" ]] || die "no VM recorded; run task azure-oidc:vm:up"
  ssh -i "$ssh_key" -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile="$vmdir/known_hosts" \
    -o ServerAliveInterval=30 "$user@$(cat "$vmdir/host")" "$@"
}

up() {
  mkdir -p "$vmdir"
  [[ -f "$ssh_key" ]] || ssh-keygen -q -t ed25519 -N "" -C rp-oidc-runner -f "$ssh_key"
  local my_ip
  my_ip="$(curl -fsS https://api.ipify.org)"
  cat > "$vmdir/cloud-init.yaml" <<'EOF'
#cloud-config
package_update: true
packages: [jq, openssl, unzip, rsync, curl, gnupg, ca-certificates]
runcmd:
  - curl -sL https://aka.ms/InstallAzureCLIDeb | bash
  - curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp.gpg
  - echo "deb [signed-by=/usr/share/keyrings/hashicorp.gpg] https://apt.releases.hashicorp.com $(. /etc/os-release && echo $VERSION_CODENAME) main" > /etc/apt/sources.list.d/hashicorp.list
  - apt-get update && apt-get install -y terraform
EOF
  echo "azure-oidc-vm: creating $vm in $resource_group ($region), SSH from $my_ip only, no managed identity"
  az group create --subscription "$subscription" -n "$resource_group" -l "$region" -o none
  az vm create --subscription "$subscription" -g "$resource_group" -n "$vm" -l "$region" \
    --image Ubuntu2404 --size Standard_D2d_v5 --admin-username "$user" \
    --ssh-key-values "$ssh_key.pub" --public-ip-sku Standard --nsg-rule NONE \
    --custom-data "$vmdir/cloud-init.yaml" -o none
  az network nsg rule create --subscription "$subscription" -g "$resource_group" --nsg-name "${vm}NSG" \
    -n ssh-from-operator --priority 100 --access Allow --protocol Tcp --direction Inbound \
    --source-address-prefixes "$my_ip/32" --destination-port-ranges 22 -o none
  az vm show -d --subscription "$subscription" -g "$resource_group" -n "$vm" --query publicIps -o tsv > "$vmdir/host"
  for _ in $(seq 1 30); do remote true 2>/dev/null && break; sleep 10; done
  remote cloud-init status --wait >/dev/null
  remote 'command -v az && command -v terraform' >/dev/null || die "cloud-init did not install az and terraform"
  echo "azure-oidc-vm: $vm ready at $(cat "$vmdir/host")"
}

check() {
  # DefaultAzureCredential hands AZURE_CLIENT_ID to ManagedIdentityCredential as
  # a user-assigned identity, and IMDS answers 400 "Identity not found" for an
  # ID the host does not carry. azidentity treats that as a hard failure, so
  # the chain stops before the Azure CLI. Any identity the host does carry is
  # irrelevant to that request. Prints the status and IMDS error only.
  [[ -n "${AZURE_CLIENT_ID:-}" ]] || die "set AZURE_CLIENT_ID"
  local out
  out="$(remote bash -s -- "$AZURE_CLIENT_ID" <<'EOF'
code="$(curl -s -o /tmp/imds -w '%{http_code}' -H Metadata:true "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https%3A%2F%2Fmanagement.azure.com%2F&client_id=$1")"
echo "$code"
if [ "$code" != 200 ]; then jq -r '.error_description // .error // empty' /tmp/imds 2>/dev/null || true; fi
rm -f /tmp/imds
EOF
)"
  echo "$out"
  [[ "$(head -n1 <<<"$out")" == 400 && "$out" == *"Identity not found"* ]] || die "IMDS did not answer 400 Identity not found for $AZURE_CLIENT_ID"
  echo "azure-oidc-vm: IMDS answers like a GitHub-hosted runner for this client ID"
}

run() {
  local action="$1" provider="$2" auth="$3"
  [[ -n "${REDPANDA_CLIENT_ID:-}" && -n "${REDPANDA_CLIENT_SECRET:-}" ]] || die "set REDPANDA_CLIENT_ID and REDPANDA_CLIENT_SECRET"
  [[ -f "$dir/issuer" ]] || die "no issuer recorded; run task azure-oidc:up"
  local stage="$vmdir/stage"
  rm -rf "$stage"
  mkdir -p "$stage/scripts" "$stage/examples/byoc/azure" "$stage/.azure-oidc/bin"
  cp scripts/azure-oidc-byoc.sh "$stage/scripts/"
  cp examples/byoc/azure/*.tf "$stage/examples/byoc/azure/"
  GOOS=linux GOARCH=amd64 go build -o "$stage/.azure-oidc/bin/azureoidc" ./cmd/azureoidc
  GOOS=linux GOARCH=amd64 go build -o "$stage/terraform-provider-redpanda" .
  "$dir/bin/azureoidc" mint -key "$dir/signing-key.pem" -issuer "$(cat "$dir/issuer")" \
    -subject "${AZURE_OIDC_SUBJECT:-tfrp-oidc-manual}" -lifetime 12h -out "$stage/token"
  (
    umask 077
    {
      for name in REDPANDA_CLIENT_ID REDPANDA_CLIENT_SECRET AZURE_CLIENT_ID AZURE_TENANT_ID; do
        printf 'export %s=%q\n' "$name" "${!name}"
      done
      printf 'export ARM_SUBSCRIPTION_ID=%q\n' "$subscription"
      printf 'export REDPANDA_CLOUD_ENVIRONMENT=%q\n' "${REDPANDA_CLOUD_ENVIRONMENT:-pre}"
      printf 'export TF_LOG=%q\n' "${TF_LOG:-DEBUG}"
      printf 'export AZURE_OIDC_NAME=%q\n' "rp-oidc-vm-$(whoami | tr -cd 'a-z0-9' | cut -c1-9)"
      printf 'export AZURE_OIDC_TOKEN_FILE=%q\n' "/home/$user/rp/token"
      printf 'export AZURE_OIDC_PROVIDER_BINARY=%q\n' "/home/$user/rp/terraform-provider-redpanda"
      [[ -n "${REDPANDA_VERSION:-}" ]] && printf 'export REDPANDA_VERSION=%q\n' "$REDPANDA_VERSION"
      for pair in ${AZURE_OIDC_VM_ENV:-}; do
        printf 'export %s=%q\n' "${pair%%=*}" "${pair#*=}"
      done
    } > "$stage/run.env"
  )
  rsync -a -e "ssh -i $ssh_key -o UserKnownHostsFile=$vmdir/known_hosts" "$stage/" "$user@$(cat "$vmdir/host"):rp/"
  rm -f "$stage/run.env" "$stage/token"
  remote "cd rp && rm -f run.done run.log && nohup setsid bash -c 'set -a; . ./run.env; set +a; ./scripts/azure-oidc-byoc.sh $action $provider $auth > run.log 2>&1; echo \$? > run.done' </dev/null >/dev/null 2>&1 &"
  echo "azure-oidc-vm: started $action $provider $auth on the VM"
  wait_run
}

wait_run() {
  remote bash -s <<'EOF'
cd rp
touch run.log
tail -n +1 -F run.log &
t=$!
while [ ! -s run.done ]; do sleep 10; done
sleep 2
kill "$t"
wait "$t" 2>/dev/null || true
rc="$(tr -cd '0-9' < run.done)"
rc="${rc:-1}"
echo "azure-oidc-vm: run exited $rc"
exit "$rc"
EOF
}

down() {
  if [[ -f "$vmdir/host" && "${FORCE:-}" != 1 ]]; then
    # The VM holds the only Terraform state for its runs, and its resources are
    # named so the sweepers skip them: an unreachable VM must stop the delete.
    local left
    left="$(remote bash -s <<'EOF'
set -o pipefail
cd rp/.azure-oidc/byoc 2>/dev/null && [ -f terraform.tfstate ] || { echo 0; exit 0; }
TF_CLI_CONFIG_FILE="$PWD/.terraformrc" terraform state list | wc -l
EOF
)" || die "could not read the VM's Terraform state; destroy through it first, or set FORCE=1 to delete it anyway"
    [[ "${left// /}" == 0 ]] || die "the VM's Terraform state still holds $left resources; destroy them first or set FORCE=1"
  fi
  if [[ "$(az group exists --subscription "$subscription" -n "$resource_group")" == "true" ]]; then
    az group delete --subscription "$subscription" -n "$resource_group" --yes
    echo "azure-oidc-vm: deleted $resource_group"
  fi
  rm -rf "$vmdir"
}

case "$cmd" in
  up) up ;;
  check) check ;;
  run) [[ $# -eq 4 ]] || die "usage: azure-oidc-vm.sh run apply|destroy local|released AUTH"; run "$2" "$3" "$4" ;;
  wait) wait_run ;;
  down) down ;;
  *) die "usage: azure-oidc-vm.sh up|check|run ACTION PROVIDER AUTH|wait|down" ;;
esac
