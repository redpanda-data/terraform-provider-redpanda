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
  throughput_tier   = "tier-1-aws-v2-arm"
  zones             = ["us-west-2a", "us-west-2b", "us-west-2c"]
}

# Dual listener mode (preview, feature-gated). connections must be set on all
# three services with the same public/private topology; auth may differ per
# service. endpoint is server-assigned. Omit connection_type when using
# connections.
resource "redpanda_cluster" "dual_listeners" {
  name              = "example-dual-listener-cluster"
  resource_group_id = redpanda_resource_group.example.id
  network_id        = redpanda_network.example.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "dedicated"
  throughput_tier   = "tier-1-aws-v2-arm"
  zones             = ["us-west-2a", "us-west-2b", "us-west-2c"]

  kafka_api = {
    connections = [
      { type = "public", auth = { mode = "sasl" } },
      { type = "private", auth = { mode = "sasl" } },
    ]
  }
  http_proxy = {
    connections = [
      { type = "public", auth = { mode = "sasl" } },
      { type = "private", auth = { mode = "sasl" } },
    ]
  }
  schema_registry = {
    connections = [
      { type = "public", auth = { mode = "sasl" } },
      { type = "private", auth = { mode = "sasl" } },
    ]
  }
}