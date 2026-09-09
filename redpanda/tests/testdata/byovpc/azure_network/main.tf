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

variable "region" {
  type    = string
  default = "eastus"
}

variable "resource_group_name" {
  type    = string
  default = "testname"
}

variable "network_name" {
  type    = string
  default = "testname"
}

variable "management_bucket_storage_account_name" { type = string }
variable "management_bucket_storage_container_name" { type = string }
variable "redpanda_resource_group_name" { type = string }
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
