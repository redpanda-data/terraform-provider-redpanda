provider "redpanda" {}

module "redpanda-byovpc" {
  source = "git::https://github.com/redpanda-data/terraform-aws-redpanda-byovpc.git?ref=add-code-from-cloudexamples"

  region = var.region
  aws_account_id = "879326078624"

  aws_access_key = var.aws_access_key
    aws_secret_key = var.aws_secret_key
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
        arn = module.redpanda-byovpc.management_bucket_arn
      }
      dynamodb_table = {
        arn = module.redpanda-byovpc.dynamodb_table_arn
      }
      vpc = {
        arn = module.redpanda-byovpc.vpc_arn
      }
      private_subnets = {
        arns = module.redpanda-byovpc.private_subnet_arns
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
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true
  tags = {
    "key" = "value"
  }
  customer_managed_resources = {
    aws = {
      aws_permissions_boundary_policy_arn = {
        arn = module.redpanda-byovpc.permissions_boundary_policy_arn
      }
      agent_instance_profile = {
        arn = module.redpanda-byovpc.agent_instance_profile_arn
      }
      connectors_node_group_instance_profile = {
        arn = module.redpanda-byovpc.connectors_node_group_instance_profile_arn
      }
      utility_node_group_instance_profile = {
        arn = module.redpanda-byovpc.utility_node_group_instance_profile_arn
      }
      redpanda_node_group_instance_profile = {
        arn = module.redpanda-byovpc.redpanda_node_group_instance_profile_arn
      }
      k8s_cluster_role = {
        arn = module.redpanda-byovpc.k8s_cluster_role_arn
      }
      redpanda_agent_security_group = {
        arn = module.redpanda-byovpc.redpanda_agent_security_group_arn
      }
      connectors_security_group = {
        arn = module.redpanda-byovpc.connectors_security_group_arn
      }
      redpanda_node_group_security_group = {
        arn = module.redpanda-byovpc.redpanda_node_group_security_group_arn
      }
      utility_security_group = {
        arn = module.redpanda-byovpc.utility_security_group_arn
      }
      cluster_security_group = {
        arn = module.redpanda-byovpc.cluster_security_group_arn
      }
      node_security_group = {
        arn = module.redpanda-byovpc.node_security_group_arn
      }
      cloud_storage_bucket = {
        arn = module.redpanda-byovpc.cloud_storage_bucket_arn
      }
      permissions_boundary_policy = {
        arn = module.redpanda-byovpc.permissions_boundary_policy_arn
      }
    }
  }
}

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
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
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