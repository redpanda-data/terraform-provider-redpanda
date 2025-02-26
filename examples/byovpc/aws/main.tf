provider "redpanda" {}

variable "management_bucket_arn" {
  default = "arn:aws:s3:::rp-879326078624-us-east-2-mgmt-20250225235918572600000009"
}

variable "dynamodb_table_arn" {
  default = "arn:aws:dynamodb:us-east-2:879326078624:table/rp-879326078624-us-east-2-mgmt-tflock-tjhc470imx"
}

variable "vpc_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:vpc/vpc-0503833a1083ea5fd"
}

variable "private_subnet_arns" {
  type = list(string)
  default = [
    "arn:aws:ec2:us-east-2:879326078624:subnet/subnet-085d9ca4d4a3b8234",
    "arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0facdafdad2b1b22e",
    "arn:aws:ec2:us-east-2:879326078624:subnet/subnet-0315e9b25f16aafdb",
    "arn:aws:ec2:us-east-2:879326078624:subnet/subnet-01165942af996f138",
    "arn:aws:ec2:us-east-2:879326078624:subnet/subnet-06885141d4dc59d85",
    "arn:aws:ec2:us-east-2:879326078624:subnet/subnet-00df5d655183e7d20"
  ]
}

variable "permissions_boundary_policy_arn" {
  default = "arn:aws:iam::879326078624:policy/redpanda-agent-boundary-20250225235919694500000019"
}

variable "agent_instance_profile_arn" {
  default = "arn:aws:iam::879326078624:instance-profile/redpanda-agent-20250225235919028600000014"
}

variable "connectors_node_group_instance_profile_arn" {
  default = "arn:aws:iam::879326078624:instance-profile/redpanda-connect-2025022523591892010000000e"
}

variable "utility_node_group_instance_profile_arn" {
  default = "arn:aws:iam::879326078624:instance-profile/redpanda-util-20250225235918953200000010"
}

variable "redpanda_node_group_instance_profile_arn" {
  default = "arn:aws:iam::879326078624:instance-profile/redpanda-rp-20250225235918976100000011"
}

variable "k8s_cluster_role_arn" {
  default = "arn:aws:iam::879326078624:role/redpanda-cluster-20250225235918054800000006"
}

variable "redpanda_agent_security_group_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:security-group/sg-0ec96ed0903732325"
}

variable "connectors_security_group_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:security-group/sg-00559db2df4b2f0b3"
}

variable "redpanda_node_group_security_group_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:security-group/sg-08cb0feabb7f8dad3"
}

variable "utility_security_group_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:security-group/sg-0a69ef8471564a7fe"
}

variable "cluster_security_group_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:security-group/sg-0d9368cd6a722a4df"
}

variable "node_security_group_arn" {
  default = "arn:aws:ec2:us-east-2:879326078624:security-group/sg-0f8d41dd76c2cb52d"
}

variable "cloud_storage_bucket_arn" {
  default = "arn:aws:s3:::redpanda-cloud-storage-20250305231154426300000001"
}

variable "byovpc_rpk_user_policy_arns" {
  default = "[]"
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