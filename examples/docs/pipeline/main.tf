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
  zones             = ["usw2-az1", "usw2-az2", "usw2-az3"]
}

resource "redpanda_pipeline" "example" {
  cluster_api_url = redpanda_cluster.example.cluster_api_url
  display_name    = "example-pipeline"
  description     = "An example Redpanda Connect pipeline"
  state           = "stopped"

  config_yaml = <<-YAML
    input:
      generate:
        interval: "1s"
        mapping: |
          root.message = "hello world"
          root.timestamp = now()

    output:
      stdout: {}
  YAML

  resources = {
    memory_shares = "256Mi"
    cpu_shares    = "200m"
  }

  tags = {
    "environment" = "example"
    "managed-by"  = "terraform"
  }
}
