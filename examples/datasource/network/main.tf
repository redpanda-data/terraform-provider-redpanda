provider "redpanda" {}

data "redpanda_network" "test" {
  id = var.network_id
}

output "network" {
  value = data.redpanda_network.test
}

output "cloud_provider" {
  value = data.redpanda_network.test.cloud_provider
}

output "cmr" {
  value = data.redpanda_network.test.customer_managed_resources
}

