terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

provider "azurerm" {
  features {
    key_vault {
      recover_soft_deleted_key_vaults = true
    }
  }
  subscription_id = var.subscription_id
  # The module disables shared-key access on its storage accounts; without
  # this the provider's data-plane calls fail with
  # KeyBasedAuthenticationNotPermitted on create and on refresh.
  storage_use_azuread = true
}

# unique_identifier keys every globally unique name the module derives.
# Storage accounts take the prefix with dashes stripped plus up to 14
# characters ("tieredstoragea") against a 24-character maximum, which caps
# the identifier at 5 characters. Key vaults keep purge protection on, so a
# soft-deleted vault holds its name for 90 days; a per-run identifier is what
# keeps a re-run from hitting VaultAlreadyExists.
variable "unique_identifier" {
  type = string

  validation {
    condition     = can(regex("^[a-z0-9]{1,5}$", var.unique_identifier))
    error_message = "unique_identifier must be 1-5 lowercase alphanumeric characters; see the comment above."
  }
}

variable "subscription_id" {
  description = "Azure subscription the module provisions into"
  type        = string
}

variable "tenant_id" {
  description = "Azure tenant of the subscription"
  type        = string
}

variable "region" {
  description = "Azure region for resources"
  type        = string
  default     = "eastus"
}

variable "zones" {
  description = "Availability zones, in Redpanda's <region>-az<n> form"
  type        = list(string)
  default     = ["eastus-az1", "eastus-az2", "eastus-az3"]
}

# The AKS service CIDR is a module input, not an output: the module reserves
# it inside the VNet and the cluster declares the same value in
# customer_managed_resources.azure.cidrs, so one variable feeds both.
variable "aks_service_cidr" {
  type    = string
  default = "10.0.15.0/24"
}

locals {
  prefix = "tfrp-${var.unique_identifier}-"
}

module "redpanda_byovnet" {
  # Git-ref pin, mirroring the AWS and GCP stacks, so CI does not silently
  # adopt a new module release.
  source = "git::https://github.com/redpanda-data/terraform-azure-redpanda-byovnet.git?ref=5b10e0db5ba17a08a515d2cdefe485b7d9a83c51"

  region                = var.region
  zones                 = var.zones
  azure_tenant_id       = var.tenant_id
  azure_subscription_id = var.subscription_id

  resource_name_prefix       = local.prefix
  resource_group_name_prefix = local.prefix
  reserved_subnet_cidrs      = { "k8s-service" = var.aks_service_cidr }

  # The BYOC agent bootstrap (rpk cloud byoc azure apply, run by the provider)
  # keeps its Terraform state in the management storage account, which allows
  # only Entra ID auth; the identity applying this module must hold Storage
  # Blob Data Contributor on it. This grants that role to the caller.
  grant_caller_management_storage_access = true

  tags = {
    "managed-by" = "terraform-provider-redpanda-acc"
    "run"        = var.unique_identifier
  }
}

# Mirrors the AWS and GCP producers: the outputs the acceptance test needs,
# written to a JSON file the task reads with jq. Every key is a leaf of
# customer_managed_resources.azure on the network or the cluster.
locals {
  azure_outputs = {
    zones            = var.zones
    aks_service_cidr = var.aks_service_cidr

    management_bucket_storage_account_name   = module.redpanda_byovnet.management_bucket_storage_account_name
    management_bucket_storage_container_name = module.redpanda_byovnet.management_bucket_storage_container_name
    vnet_name                                = module.redpanda_byovnet.vnet_name
    network_resource_group_name              = module.redpanda_byovnet.network_resource_group_name

    rp_agent_subnet_name           = module.redpanda_byovnet.agent_private_subnet_name
    rp_0_pods_subnet_name          = module.redpanda_byovnet.rp_0_pods_subnet_name
    rp_0_vnet_subnet_name          = module.redpanda_byovnet.rp_0_vnet_subnet_name
    rp_1_pods_subnet_name          = module.redpanda_byovnet.rp_1_pods_subnet_name
    rp_1_vnet_subnet_name          = module.redpanda_byovnet.rp_1_vnet_subnet_name
    rp_2_pods_subnet_name          = module.redpanda_byovnet.rp_2_pods_subnet_name
    rp_2_vnet_subnet_name          = module.redpanda_byovnet.rp_2_vnet_subnet_name
    rp_connect_pods_subnet_name    = module.redpanda_byovnet.rp_connect_pods_subnet_name
    rp_connect_vnet_subnet_name    = module.redpanda_byovnet.rp_connect_vnet_subnet_name
    kafka_connect_pods_subnet_name = module.redpanda_byovnet.kafka_connect_pods_subnet_name
    kafka_connect_vnet_subnet_name = module.redpanda_byovnet.kafka_connect_vnet_subnet_name
    sys_pods_subnet_name           = module.redpanda_byovnet.system_pods_subnet_name
    sys_vnet_subnet_name           = module.redpanda_byovnet.system_vnet_subnet_name
    rp_egress_vnet_subnet_name     = module.redpanda_byovnet.egress_subnet_name

    redpanda_resource_group_name = module.redpanda_byovnet.redpanda_resource_group_name
    storage_resource_group_name  = module.redpanda_byovnet.storage_resource_group_name
    iam_resource_group_name      = module.redpanda_byovnet.iam_resource_group_name

    agent_user_assigned_identity_name           = module.redpanda_byovnet.agent_user_assigned_identity_name
    aks_user_assigned_identity_name             = module.redpanda_byovnet.aks_user_assigned_identity_name
    redpanda_cluster_assigned_identity_name     = module.redpanda_byovnet.cluster_user_assigned_identity_name
    cert_manager_assigned_identity_name         = module.redpanda_byovnet.cert_manager_user_assigned_identity_name
    external_dns_assigned_identity_name         = module.redpanda_byovnet.external_dns_user_assigned_identity_name
    redpanda_console_assigned_identity_name     = module.redpanda_byovnet.console_user_assigned_identity_name
    kafka_connect_assigned_identity_name        = module.redpanda_byovnet.kafka_connect_user_assigned_identity_name
    redpanda_connect_assigned_identity_name     = module.redpanda_byovnet.redpanda_connect_user_assigned_identity_name
    redpanda_connect_api_assigned_identity_name = module.redpanda_byovnet.redpanda_connect_api_user_assigned_identity_name
    redpanda_operator_assigned_identity_name    = module.redpanda_byovnet.redpanda_operator_user_assigned_identity_name

    tiered_storage_account_name   = module.redpanda_byovnet.tiered_storage_account_name
    tiered_storage_container_name = module.redpanda_byovnet.tiered_storage_container_name
    management_key_vault_name     = module.redpanda_byovnet.management_key_vault_name
    console_key_vault_name        = module.redpanda_byovnet.console_key_vault_name
    redpanda_security_group_name  = module.redpanda_byovnet.redpanda_security_group_name
  }
}

resource "local_file" "inputs_json" {
  content  = jsonencode(local.azure_outputs)
  filename = "${path.module}/inputs.json"
}

output "azure_outputs" {
  value = local.azure_outputs
}
