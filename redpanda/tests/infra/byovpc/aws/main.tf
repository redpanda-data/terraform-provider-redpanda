variable "resource_group_name" {
  default = ""
}
resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

variable "network_name" {
}
variable "cloud_provider" {
}
variable "region" {
}
variable "management_bucket_arn" {
}
variable "dynamodb_table_arn" {
}
variable "vpc_arn" {
}
variable "private_subnet_arns" {
  type = list(string)
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
        arn = var.management_bucket_arn
      }
      dynamodb_table = {
        arn = var.dynamodb_table_arn
      }
      vpc = {
        arn = var.vpc_arn
      }
      private_subnets = {
        arns = var.private_subnet_arns
      }
    }
  }
}

variable "cluster_name" {
}
variable "throughput_tier" {
  default = "tier-1-aws-v2-x86"
}

variable "zones" {
  type = list(string)
}
variable "permissions_boundary_policy_arn" {
}
variable "agent_instance_profile_arn" {
}
variable "connectors_node_group_instance_profile_arn" {
}
variable "utility_node_group_instance_profile_arn" {
}
variable "redpanda_node_group_instance_profile_arn" {
}
variable "k8s_cluster_role_arn" {
}
variable "redpanda_agent_security_group_arn" {
}
variable "connectors_security_group_arn" {
}
variable "redpanda_node_group_security_group_arn" {
}
variable "utility_security_group_arn" {
}
variable "cluster_security_group_arn" {
}
variable "node_security_group_arn" {
}
variable "cloud_storage_bucket_arn" {
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
      aws_private_link = {
        enabled         = true
        connect_console = true
        allowed_principals = ["arn:aws:iam::879326078624:root"]
      }
      aws_permissions_boundary_policy_arn = {
        arn = var.permissions_boundary_policy_arn
      }
      agent_instance_profile = {
        arn = var.agent_instance_profile_arn
      }
      connectors_node_group_instance_profile = {
        arn = var.connectors_node_group_instance_profile_arn
      }
      utility_node_group_instance_profile = {
        arn = var.utility_node_group_instance_profile_arn
      }
      redpanda_node_group_instance_profile = {
        arn = var.redpanda_node_group_instance_profile_arn
      }
      k8s_cluster_role = {
        arn = var.k8s_cluster_role_arn
      }
      redpanda_agent_security_group = {
        arn = var.redpanda_agent_security_group_arn
      }
      connectors_security_group = {
        arn = var.connectors_security_group_arn
      }
      redpanda_node_group_security_group = {
        arn = var.redpanda_node_group_security_group_arn
      }
      utility_security_group = {
        arn = var.utility_security_group_arn
      }
      cluster_security_group = {
        arn = var.cluster_security_group_arn
      }
      node_security_group = {
        arn = var.node_security_group_arn
      }
      cloud_storage_bucket = {
        arn = var.cloud_storage_bucket_arn
      }
      permissions_boundary_policy = {
        arn = var.permissions_boundary_policy_arn
      }
    }
  }
}

#
# resource "redpanda_user" "test" {
#   name            = var.user_name
#   password        = var.user_pw
#   mechanism       = var.mechanism
#   cluster_api_url = redpanda_cluster.test.cluster_api_url
# }
#
# resource "redpanda_topic" "test" {
#   name               = var.topic_name
#   partition_count    = var.partition_count
#   replication_factor = var.replication_factor
#   cluster_api_url    = redpanda_cluster.test.cluster_api_url
#   allow_deletion     = true
# }
#
#
# resource "redpanda_acl" "test" {
#   resource_type         = "TOPIC"
#   resource_name         = redpanda_topic.test.name
#   resource_pattern_type = "LITERAL"
#   principal             = "User:${redpanda_user.test.name}"
#   host                  = "*"
#   operation             = "READ"
#   permission_type       = "ALLOW"
#   cluster_api_url       = redpanda_cluster.test.cluster_api_url
# }
#
#
# variable "user_name" {
#   default = "test-username"
# }
#
# variable "user_pw" {
#   default = "password"
# }
#
# variable "mechanism" {
#   default = "scram-sha-256"
# }
#
# variable "topic_name" {
#   default = "test-topic"
# }
#
# variable "partition_count" {
#   default = 3
# }
#
# variable "replication_factor" {
#   default = 3
# }
