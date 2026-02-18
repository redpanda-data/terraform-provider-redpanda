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
        arn = module.redpanda_byovpc.management_bucket_arn
      }
      dynamodb_table = {
        arn = module.redpanda_byovpc.dynamodb_table_arn
      }
      vpc = {
        arn = module.redpanda_byovpc.vpc_arn
      }
      private_subnets = {
        arns = module.redpanda_byovpc.private_subnet_arns
      }
    }
  }
  depends_on = [
    module.redpanda_byovpc
  ]
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
        arn = module.redpanda_byovpc.permissions_boundary_policy_arn
      }
      agent_instance_profile = {
        arn = module.redpanda_byovpc.agent_instance_profile_arn
      }
      connectors_node_group_instance_profile = {
        arn = module.redpanda_byovpc.connectors_node_group_instance_profile_arn
      }
      utility_node_group_instance_profile = {
        arn = module.redpanda_byovpc.utility_node_group_instance_profile_arn
      }
      redpanda_node_group_instance_profile = {
        arn = module.redpanda_byovpc.redpanda_node_group_instance_profile_arn
      }
      k8s_cluster_role = {
        arn = module.redpanda_byovpc.k8s_cluster_role_arn
      }
      redpanda_agent_security_group = {
        arn = module.redpanda_byovpc.redpanda_agent_security_group_arn
      }
      connectors_security_group = {
        arn = module.redpanda_byovpc.connectors_security_group_arn
      }
      redpanda_node_group_security_group = {
        arn = module.redpanda_byovpc.redpanda_node_group_security_group_arn
      }
      utility_security_group = {
        arn = module.redpanda_byovpc.utility_security_group_arn
      }
      cluster_security_group = {
        arn = module.redpanda_byovpc.cluster_security_group_arn
      }
      node_security_group = {
        arn = module.redpanda_byovpc.node_security_group_arn
      }
      cloud_storage_bucket = {
        arn = module.redpanda_byovpc.cloud_storage_bucket_arn
      }
      permissions_boundary_policy = {
        arn = module.redpanda_byovpc.permissions_boundary_policy_arn
      }
    }
  }
  depends_on = [
    redpanda_network.test
  ]
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = var.user_allow_deletion
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
  allow_deletion        = var.acl_allow_deletion
}
