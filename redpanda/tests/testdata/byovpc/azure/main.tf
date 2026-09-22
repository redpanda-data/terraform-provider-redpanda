provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "azure"
  region            = var.region
  cluster_type      = "byoc"

  customer_managed_resources = {
    azure = {
      management_bucket = {
        storage_account_name   = var.management_bucket_storage_account_name
        storage_container_name = var.management_bucket_storage_container_name
        resource_group         = { name = var.redpanda_resource_group_name }
      }
      vnet = {
        name           = var.vnet_name
        resource_group = { name = var.network_resource_group_name }
      }
      subnets = {
        rp_agent           = { name = var.rp_agent_subnet_name }
        rp_0_pods          = { name = var.rp_0_pods_subnet_name }
        rp_0_vnet          = { name = var.rp_0_vnet_subnet_name }
        rp_1_pods          = { name = var.rp_1_pods_subnet_name }
        rp_1_vnet          = { name = var.rp_1_vnet_subnet_name }
        rp_2_pods          = { name = var.rp_2_pods_subnet_name }
        rp_2_vnet          = { name = var.rp_2_vnet_subnet_name }
        rp_connect_pods    = { name = var.rp_connect_pods_subnet_name }
        rp_connect_vnet    = { name = var.rp_connect_vnet_subnet_name }
        kafka_connect_pods = { name = var.kafka_connect_pods_subnet_name }
        kafka_connect_vnet = { name = var.kafka_connect_vnet_subnet_name }
        sys_pods           = { name = var.sys_pods_subnet_name }
        sys_vnet           = { name = var.sys_vnet_subnet_name }
        rp_egress_vnet     = { name = var.rp_egress_vnet_subnet_name }
      }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
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

# Dataplane resources stay out: a BYOVPC dataplane is reachable only from
# inside the VNet, which CI cannot offer. Dataplane coverage lives on
# serverless.

variable "region" {
  description = "Azure region for resources"
  type        = string
  default     = "eastus"
}

variable "resource_group_name" {
  description = "Redpanda resource group name"
  type        = string
  default     = "testname"
}

variable "network_name" {
  description = "Name for the Redpanda network"
  type        = string
  default     = "testname"
}

variable "cluster_name" {
  description = "Name for the Redpanda cluster"
  type        = string
  default     = "testname"
}

variable "throughput_tier" {
  description = "Throughput tier for the Redpanda cluster"
  type        = string
  default     = "tier-1-azure-v3-x86"
}

variable "zones" {
  description = "Azure zones for the Redpanda cluster"
  type        = list(string)
  default     = ["eastus-az1", "eastus-az2", "eastus-az3"]
}

variable "cluster_tags" {
  type    = map(string)
  default = { "managed-by" = "terraform-provider-redpanda-acc" }
}

variable "management_bucket_storage_account_name" { type = string }
variable "management_bucket_storage_container_name" { type = string }
variable "vnet_name" { type = string }
variable "network_resource_group_name" { type = string }

variable "rp_agent_subnet_name" { type = string }
variable "rp_0_pods_subnet_name" { type = string }
variable "rp_0_vnet_subnet_name" { type = string }
variable "rp_1_pods_subnet_name" { type = string }
variable "rp_1_vnet_subnet_name" { type = string }
variable "rp_2_pods_subnet_name" { type = string }
variable "rp_2_vnet_subnet_name" { type = string }
variable "rp_connect_pods_subnet_name" { type = string }
variable "rp_connect_vnet_subnet_name" { type = string }
variable "kafka_connect_pods_subnet_name" { type = string }
variable "kafka_connect_vnet_subnet_name" { type = string }
variable "sys_pods_subnet_name" { type = string }
variable "sys_vnet_subnet_name" { type = string }
variable "rp_egress_vnet_subnet_name" { type = string }

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
