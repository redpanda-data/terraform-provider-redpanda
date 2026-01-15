provider "redpanda" {}

resource "redpanda_resource_group" "example" {
  name = var.resource_group_name
}

resource "redpanda_serverless_private_link" "example" {
  name               = var.private_link_name
  resource_group_id  = redpanda_resource_group.example.id
  cloud_provider     = "aws"
  serverless_region  = var.serverless_region
  allow_deletion     = var.allow_deletion

  cloud_provider_config = {
    aws = {
      allowed_principals = var.allowed_principals
    }
  }
}

output "private_link_id" {
  value       = redpanda_serverless_private_link.example.id
  description = "The ID of the serverless private link"
}

output "vpc_endpoint_service_name" {
  value       = redpanda_serverless_private_link.example.status.aws.vpc_endpoint_service_name
  description = "VPC endpoint service name for creating VPC endpoints"
}

output "availability_zones" {
  value       = redpanda_serverless_private_link.example.status.aws.availability_zones
  description = "Availability zones where the private link service is available"
}

output "state" {
  value       = redpanda_serverless_private_link.example.state
  description = "Current state of the serverless private link"
}
