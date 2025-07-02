variable "region" {
  default = "us-east-2"
}
provider aws {
  region = var.region
}

module "redpanda_byovpc" {
  source = "redpanda-data/redpanda-byovpc/aws"

  region = var.region
  public_subnet_cidrs = [
    "10.0.16.0/20", "10.0.32.0/20", "10.0.48.0/20"
  ]
  enable_private_link = true
  enable_redpanda_connect = true
}

# Capture all ARN outputs from the module
locals {
  arn_outputs = {
    management_bucket_arn                        = module.redpanda_byovpc.management_bucket_arn
    dynamodb_table_arn                           = module.redpanda_byovpc.dynamodb_table_arn
    vpc_arn                                      = module.redpanda_byovpc.vpc_arn
    private_subnet_arns                          = jsonencode(module.redpanda_byovpc.private_subnet_arns)
    permissions_boundary_policy_arn              = module.redpanda_byovpc.permissions_boundary_policy_arn
    agent_instance_profile_arn                   = module.redpanda_byovpc.agent_instance_profile_arn
    connectors_node_group_instance_profile_arn   = module.redpanda_byovpc.connectors_node_group_instance_profile_arn
    utility_node_group_instance_profile_arn      = module.redpanda_byovpc.utility_node_group_instance_profile_arn
    redpanda_node_group_instance_profile_arn     = module.redpanda_byovpc.redpanda_node_group_instance_profile_arn
    k8s_cluster_role_arn                         = module.redpanda_byovpc.k8s_cluster_role_arn
    redpanda_agent_security_group_arn            = module.redpanda_byovpc.redpanda_agent_security_group_arn
    connectors_security_group_arn                = module.redpanda_byovpc.connectors_security_group_arn
    redpanda_node_group_security_group_arn       = module.redpanda_byovpc.redpanda_node_group_security_group_arn
    utility_security_group_arn                   = module.redpanda_byovpc.utility_security_group_arn
    cluster_security_group_arn                   = module.redpanda_byovpc.cluster_security_group_arn
    node_security_group_arn                      = module.redpanda_byovpc.node_security_group_arn
    cloud_storage_bucket_arn                     = module.redpanda_byovpc.cloud_storage_bucket_arn
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