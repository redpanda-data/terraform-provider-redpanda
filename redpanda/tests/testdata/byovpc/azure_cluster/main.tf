provider "redpanda" {}

# The resource group and the BYOVNet network are created by the test through
# the control-plane API and passed in by id, so this config exercises the
# cluster's customer_managed_resources.azure alone.
resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = var.resource_group_id
  network_id        = var.network_id
  cloud_provider    = "azure"
  region            = var.region
  cluster_type      = "byoc"
  connection_type   = "private"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true

  tags = var.cluster_tags

  customer_managed_resources = {
    azure = {
      resource_groups = {
        redpanda_resource_group = { name = var.redpanda_resource_group_name }
        storage_resource_group  = { name = var.storage_resource_group_name }
        iam_resource_group      = { name = var.iam_resource_group_name }
      }
      user_assigned_identities = {
        agent_user_assigned_identity           = { name = var.agent_user_assigned_identity_name }
        aks_user_assigned_identity             = { name = var.aks_user_assigned_identity_name }
        redpanda_cluster_assigned_identity     = { name = var.redpanda_cluster_assigned_identity_name }
        cert_manager_assigned_identity         = { name = var.cert_manager_assigned_identity_name }
        external_dns_assigned_identity         = { name = var.external_dns_assigned_identity_name }
        redpanda_console_assigned_identity     = { name = var.redpanda_console_assigned_identity_name }
        kafka_connect_assigned_identity        = { name = var.kafka_connect_assigned_identity_name }
        redpanda_connect_assigned_identity     = { name = var.redpanda_connect_assigned_identity_name }
        redpanda_connect_api_assigned_identity = { name = var.redpanda_connect_api_assigned_identity_name }
        redpanda_operator_assigned_identity    = { name = var.redpanda_operator_assigned_identity_name }
      }
      tiered_cloud_storage = {
        storage_account_name   = var.tiered_storage_account_name
        storage_container_name = var.tiered_storage_container_name
        resource_group         = { name = var.storage_resource_group_name }
      }
      key_vaults = {
        management_vault = { name = var.management_key_vault_name }
        console_vault    = { name = var.console_key_vault_name }
      }
      security_groups = {
        redpanda_security_group = { name = var.redpanda_security_group_name }
      }
      cidrs = {
        aks_service_cidr = var.aks_service_cidr
      }
    }
  }

  # Azure BYOC agent teardown can outlast the provider's default delete
  # budget; create is sized like the examples.
  timeouts = {
    create = "150m"
    delete = "150m"
  }
}

variable "resource_group_id" { type = string }
variable "network_id" { type = string }

variable "region" {
  type    = string
  default = "eastus"
}

variable "cluster_name" {
  type    = string
  default = "testname"
}

variable "throughput_tier" {
  type    = string
  default = "tier-1-azure-v3-x86"
}

variable "zones" {
  type    = list(string)
  default = ["eastus-az1", "eastus-az2", "eastus-az3"]
}

variable "cluster_tags" {
  type    = map(string)
  default = { "managed-by" = "terraform-provider-redpanda-acc" }
}

variable "redpanda_resource_group_name" { type = string }
variable "storage_resource_group_name" { type = string }
variable "iam_resource_group_name" { type = string }

variable "agent_user_assigned_identity_name" { type = string }
variable "aks_user_assigned_identity_name" { type = string }
variable "redpanda_cluster_assigned_identity_name" { type = string }
variable "cert_manager_assigned_identity_name" { type = string }
variable "external_dns_assigned_identity_name" { type = string }
variable "redpanda_console_assigned_identity_name" { type = string }
variable "kafka_connect_assigned_identity_name" { type = string }
variable "redpanda_connect_assigned_identity_name" { type = string }
variable "redpanda_connect_api_assigned_identity_name" { type = string }
variable "redpanda_operator_assigned_identity_name" { type = string }

variable "tiered_storage_account_name" { type = string }
variable "tiered_storage_container_name" { type = string }
variable "management_key_vault_name" { type = string }
variable "console_key_vault_name" { type = string }
variable "redpanda_security_group_name" { type = string }
variable "aks_service_cidr" { type = string }
