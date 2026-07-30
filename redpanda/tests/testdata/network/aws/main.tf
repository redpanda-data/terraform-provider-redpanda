variable "region" {
  default = "us-east-2"
}
provider "aws" {
  region = var.region
}

data "aws_availability_zones" "available" {
  state = "available"
  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

module "redpanda_byovpc" {
  // Git-ref pin: the module's registry releases predate its rpsql support
  // (enable_redpanda_sql). Re-pin to a registry version once a release
  // containing that variable ships.
  source = "git::https://github.com/redpanda-data/terraform-aws-redpanda-byovpc.git?ref=545e1eccbcea67469afeb9e10a8310590c998d23"

  region = var.region
  public_subnet_cidrs = [
    "10.0.16.0/20", "10.0.32.0/20", "10.0.48.0/20"
  ]
  enable_private_link     = true
  enable_redpanda_connect = true
  enable_redpanda_sql     = true
  default_tags = {
    Name = "redpanda-byovpc-test"
  }
}

# Capture all ARN outputs from the module
locals {
  arn_outputs = {
    management_bucket_arn                      = module.redpanda_byovpc.management_bucket_arn
    dynamodb_table_arn                         = module.redpanda_byovpc.dynamodb_table_arn
    vpc_arn                                    = module.redpanda_byovpc.vpc_arn
    private_subnet_arns                        = jsonencode(module.redpanda_byovpc.private_subnet_arns)
    zones                                      = jsonencode(slice(data.aws_availability_zones.available.zone_ids, 0, 3))
    permissions_boundary_policy_arn            = module.redpanda_byovpc.permissions_boundary_policy_arn
    agent_instance_profile_arn                 = module.redpanda_byovpc.agent_instance_profile_arn
    connectors_node_group_instance_profile_arn = module.redpanda_byovpc.connectors_node_group_instance_profile_arn
    utility_node_group_instance_profile_arn    = module.redpanda_byovpc.utility_node_group_instance_profile_arn
    redpanda_node_group_instance_profile_arn   = module.redpanda_byovpc.redpanda_node_group_instance_profile_arn
    k8s_cluster_role_arn                       = module.redpanda_byovpc.k8s_cluster_role_arn
    redpanda_agent_security_group_arn          = module.redpanda_byovpc.redpanda_agent_security_group_arn
    connectors_security_group_arn              = module.redpanda_byovpc.connectors_security_group_arn
    redpanda_node_group_security_group_arn     = module.redpanda_byovpc.redpanda_node_group_security_group_arn
    utility_security_group_arn                 = module.redpanda_byovpc.utility_security_group_arn
    cluster_security_group_arn                 = module.redpanda_byovpc.cluster_security_group_arn
    node_security_group_arn                    = module.redpanda_byovpc.node_security_group_arn
    cloud_storage_bucket_arn                   = module.redpanda_byovpc.cloud_storage_bucket_arn
    rpsql_cloud_storage_bucket_arn             = module.redpanda_byovpc.rpsql_cloud_storage_bucket_arn
    rpsql_node_group_instance_profile_arn      = module.redpanda_byovpc.rpsql_node_group_instance_profile_arn
    rpsql_security_group_arn                   = module.redpanda_byovpc.rpsql_security_group_arn
  }
}

# Write the outputs to a JSON file
resource "local_file" "inputs_json" {
  content  = jsonencode(local.arn_outputs)
  filename = "${path.module}/inputs.json"
}

# Also expose these as outputs for verification
output "arn_outputs" {
  value = local.arn_outputs
}