provider "redpanda" {}

variable "user_password" {
  type        = string
  sensitive   = true
  description = "Password for the Redpanda user"
}

resource "redpanda_resource_group" "example" {
  name = "example-resource-group"
}

resource "redpanda_network" "example" {
  name              = "example-network"
  resource_group_id = redpanda_resource_group.example.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "example" {
  name              = "example-cluster"
  resource_group_id = redpanda_resource_group.example.id
  network_id        = redpanda_network.example.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = "tier-1-aws"
  zones             = ["us-west-2a", "us-west-2b", "us-west-2c"]
}

resource "redpanda_user" "example" {
  name                = "example-user"
  password_wo         = var.user_password # Write-only, not stored in state
  password_wo_version = 1                 # Increment to trigger password update
  mechanism           = "scram-sha-256"
  cluster_api_url     = redpanda_cluster.example.cluster_api_url
  allow_deletion      = true
}