provider "redpanda" {}

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
  name            = "schema-user"
  password        = "secure-password-123"
  mechanism       = "scram-sha-256"
  cluster_api_url = redpanda_cluster.example.cluster_api_url
  allow_deletion  = true
}

resource "redpanda_schema" "example" {
  cluster_id  = redpanda_cluster.example.id
  subject     = "user-value"
  schema_type = "AVRO"
  schema = jsonencode({
    type = "record"
    name = "User"
    fields = [
      {
        name = "id"
        type = "long"
      },
      {
        name = "username"
        type = "string"
      },
      {
        name = "email"
        type = "string"
      }
    ]
  })
  username = redpanda_user.example.name
  password = "secure-password-123"
}