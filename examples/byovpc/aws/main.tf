provider "redpanda" {}

provider aws {
    region = var.region
}

module "redpanda_byovpc" {
  source = "redpanda-data/redpanda-byovpc/aws"

  region = var.region
  public_subnet_cidrs = [
    "10.0.16.0/20", "10.0.32.0/20", "10.0.48.0/20"
  ]
}


resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "byoc"
  customer_managed_resources = {
    aws = {
      management_bucket = {
        arn = redpanda_byovpc.management_bucket_arn
      }
      dynamodb_table = {
        arn = red
      }
      vpc = {
        arn = redpanda_byovpc.vpc_arn
      }
      private_subnets = {
        arns = redpanda_byovpc.private_subnet_arns
      }
    }
  }
}

resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = redpanda_network.test.cloud_provider
  region            = redpanda_network.test.region
  cluster_type      = redpanda_network.test.cluster_type
  connection_type   = "private"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true
  tags = {
    "key" = "value"
  }
  customer_managed_resources = {
    aws = {
      aws_permissions_boundary_policy_arn = {
        arn = redpanda_byovpc.permissions_boundary_policy_arn
      }
      agent_instance_profile = {
        arn = redpanda_byovpc.agent_instance_profile_arn
      }
      connectors_node_group_instance_profile = {
        arn = redpanda_byovpc.connectors_node_group_instance_profile_arn
      }
      utility_node_group_instance_profile = {
        arn = redpanda_byovpc.utility_node_group_instance_profile_arn
      }
      redpanda_node_group_instance_profile = {
        arn = redpanda_byovpc.redpanda_node_group_instance_profile_arn
      }
      k8s_cluster_role = {
        arn = redpanda_byovpc.k8s_cluster_role_arn
      }
      redpanda_agent_security_group = {
        arn = redpanda_byovpc.redpanda_agent_security_group_arn
      }
      connectors_security_group = {
        arn = redpanda_byovpc.connectors_security_group_arn
      }
      redpanda_node_group_security_group = {
        arn = redpanda_byovpc.redpanda_node_group_security_group_arn
      }
      utility_security_group = {
        arn = redpanda_byovpc.utility_security_group_arn
      }
      cluster_security_group = {
        arn = redpanda_byovpc.cluster_security_group_arn
      }
      node_security_group = {
        arn = redpanda_byovpc.node_security_group_arn
      }
      cloud_storage_bucket = {
        arn = redpanda_byovpc.cloud_storage_bucket_arn
      }
      permissions_boundary_policy = {
        arn = redpanda_byovpc.permissions_boundary_policy_arn
      }
    }
  }
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}

resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation            = "READ"
  permission_type      = "ALLOW"
  cluster_api_url      = redpanda_cluster.test.cluster_api_url
}


# Existing variables from original configuration
variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-x86"
}

variable "user_name" {
  default = "test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}

variable "aws_access_key" {
  type = string
}

variable "aws_secret_key" {
  type = string
}
