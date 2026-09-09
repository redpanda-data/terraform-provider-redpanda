provider "azurerm" {
  features {}
  subscription_id = var.azure_subscription_id
  # The module disables shared-key access on its storage accounts; without
  # this the provider's post-create data-plane check fails with
  # KeyBasedAuthenticationNotPermitted.
  storage_use_azuread = true
}

provider "redpanda" {}

# The AKS service CIDR is a module input, not an output: the module reserves
# it inside the VNet and the cluster declares the same value, so one local
# feeds both.
locals {
  aks_service_cidr = "10.0.15.0/24"
}

# Use the Redpanda Azure BYOVNet module
module "redpanda_byovnet" {
  source  = "redpanda-data/redpanda-byovnet/azure"
  version = "~> 1.1"

  region                = var.region
  azure_tenant_id       = var.azure_tenant_id
  azure_subscription_id = var.azure_subscription_id

  resource_name_prefix       = var.resource_name_prefix
  resource_group_name_prefix = var.resource_group_name_prefix
  reserved_subnet_cidrs      = { "k8s-service" = local.aks_service_cidr }

  # The BYOC agent bootstrap (rpk cloud byoc azure apply, run by the provider)
  # keeps its Terraform state in the management storage account, which allows
  # only Entra ID auth; the identity applying this module must hold Storage
  # Blob Data Contributor on it. This grants that role to the caller.
  grant_caller_management_storage_access = true

  tags = {
    "environment" = var.environment
    "managed-by"  = "terraform"
  }
}

# Redpanda resource group
resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

# Create Redpanda network with customer managed resources
resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "azure"
  region            = var.region
  cluster_type      = "byoc"

  customer_managed_resources = {
    azure = {
      management_bucket = {
        storage_account_name   = module.redpanda_byovnet.management_bucket_storage_account_name
        storage_container_name = module.redpanda_byovnet.management_bucket_storage_container_name
        resource_group         = { name = module.redpanda_byovnet.redpanda_resource_group_name }
      }
      vnet = {
        name           = module.redpanda_byovnet.vnet_name
        resource_group = { name = module.redpanda_byovnet.network_resource_group_name }
      }
      subnets = {
        rp_agent           = { name = module.redpanda_byovnet.agent_private_subnet_name }
        rp_0_pods          = { name = module.redpanda_byovnet.rp_0_pods_subnet_name }
        rp_0_vnet          = { name = module.redpanda_byovnet.rp_0_vnet_subnet_name }
        rp_1_pods          = { name = module.redpanda_byovnet.rp_1_pods_subnet_name }
        rp_1_vnet          = { name = module.redpanda_byovnet.rp_1_vnet_subnet_name }
        rp_2_pods          = { name = module.redpanda_byovnet.rp_2_pods_subnet_name }
        rp_2_vnet          = { name = module.redpanda_byovnet.rp_2_vnet_subnet_name }
        rp_connect_pods    = { name = module.redpanda_byovnet.rp_connect_pods_subnet_name }
        rp_connect_vnet    = { name = module.redpanda_byovnet.rp_connect_vnet_subnet_name }
        kafka_connect_pods = { name = module.redpanda_byovnet.kafka_connect_pods_subnet_name }
        kafka_connect_vnet = { name = module.redpanda_byovnet.kafka_connect_vnet_subnet_name }
        sys_pods           = { name = module.redpanda_byovnet.system_pods_subnet_name }
        sys_vnet           = { name = module.redpanda_byovnet.system_vnet_subnet_name }
        rp_egress_vnet     = { name = module.redpanda_byovnet.egress_subnet_name }
      }
    }
  }
  depends_on = [module.redpanda_byovnet]
}

# Create Redpanda cluster with customer managed resources
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

  tags = {
    "environment" = var.environment
    "managed-by"  = "terraform"
  }

  # Customer managed resources for Azure. Every leaf is immutable on the
  # control plane: changing one plans a replacement of the cluster.
  customer_managed_resources = {
    azure = {
      resource_groups = {
        redpanda_resource_group = { name = module.redpanda_byovnet.redpanda_resource_group_name }
        storage_resource_group  = { name = module.redpanda_byovnet.storage_resource_group_name }
        iam_resource_group      = { name = module.redpanda_byovnet.iam_resource_group_name }
      }
      user_assigned_identities = {
        agent_user_assigned_identity           = { name = module.redpanda_byovnet.agent_user_assigned_identity_name }
        aks_user_assigned_identity             = { name = module.redpanda_byovnet.aks_user_assigned_identity_name }
        redpanda_cluster_assigned_identity     = { name = module.redpanda_byovnet.cluster_user_assigned_identity_name }
        cert_manager_assigned_identity         = { name = module.redpanda_byovnet.cert_manager_user_assigned_identity_name }
        external_dns_assigned_identity         = { name = module.redpanda_byovnet.external_dns_user_assigned_identity_name }
        redpanda_console_assigned_identity     = { name = module.redpanda_byovnet.console_user_assigned_identity_name }
        kafka_connect_assigned_identity        = { name = module.redpanda_byovnet.kafka_connect_user_assigned_identity_name }
        redpanda_connect_assigned_identity     = { name = module.redpanda_byovnet.redpanda_connect_user_assigned_identity_name }
        redpanda_connect_api_assigned_identity = { name = module.redpanda_byovnet.redpanda_connect_api_user_assigned_identity_name }
        redpanda_operator_assigned_identity    = { name = module.redpanda_byovnet.redpanda_operator_user_assigned_identity_name }
      }
      tiered_cloud_storage = {
        storage_account_name   = module.redpanda_byovnet.tiered_storage_account_name
        storage_container_name = module.redpanda_byovnet.tiered_storage_container_name
        resource_group         = { name = module.redpanda_byovnet.storage_resource_group_name }
      }
      key_vaults = {
        management_vault = { name = module.redpanda_byovnet.management_key_vault_name }
        console_vault    = { name = module.redpanda_byovnet.console_key_vault_name }
      }
      security_groups = {
        redpanda_security_group = { name = module.redpanda_byovnet.redpanda_security_group_name }
      }
      cidrs = {
        aks_service_cidr = local.aks_service_cidr
      }
    }
  }

  depends_on = [redpanda_network.test]
}
