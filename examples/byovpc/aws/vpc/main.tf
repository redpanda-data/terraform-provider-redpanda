module "redpanda-byovpc" {
  source = "git::https://github.com/redpanda-data/terraform-aws-redpanda-byovpc.git?ref=add-code-from-cloudexamples"

  region = var.region
  aws_account_id = "879326078624"

  aws_access_key = var.aws_access_key_id
  aws_secret_key = var.aws_secret_access_key
  public_subnet_cidrs = [
    "10.0.16.0/20", "10.0.32.0/20", "10.0.48.0/20"
  ]
}

variable "aws_access_key_id" {
  default = "access_key_id"
  type = string
}

variable "aws_secret_access_key" {
  default = "secret_key_id"
  type = string
}

variable "region" {
  type        = string
  default = "us-east-2"
}

output "management_bucket_arn" {
  value = module.redpanda-byovpc.management_bucket_arn
}

output "dynamodb_table_arn" {
  value = module.redpanda-byovpc.dynamodb_table_arn
}

output "vpc_arn" {
  value = module.redpanda-byovpc.vpc_arn
}

output "private_subnet_arns" {
  value = module.redpanda-byovpc.private_subnet_arns
}

output "permissions_boundary_policy_arn" {
  value = module.redpanda-byovpc.permissions_boundary_policy_arn
}

output "agent_instance_profile_arn" {
  value = module.redpanda-byovpc.agent_instance_profile_arn
}

output "cloud_storage_bucket_arn" {
  value = module.redpanda-byovpc.cloud_storage_bucket_arn
}

output "redpanda_agent_security_group_arn" {
  value = module.redpanda-byovpc.redpanda_agent_security_group_arn
}

output "connectors_security_group_arn" {
  value = module.redpanda-byovpc.connectors_security_group_arn
}

output "redpanda_node_group_security_group_arn" {
  value = module.redpanda-byovpc.redpanda_node_group_security_group_arn
}

output "utility_security_group_arn" {
  value = module.redpanda-byovpc.utility_security_group_arn
}

output "cluster_security_group_arn" {
  value = module.redpanda-byovpc.cluster_security_group_arn
}

output "node_security_group_arn" {
  value = module.redpanda-byovpc.node_security_group_arn
}

output "byovpc_rpk_user_policy_arns" {
  value = module.redpanda-byovpc.byovpc_rpk_user_policy_arns
}

output "connectors_node_group_instance_profile_arn" {
  value = module.redpanda-byovpc.connectors_node_group_instance_profile_arn
}

output "utility_node_group_instance_profile_arn" {
  value = module.redpanda-byovpc.utility_node_group_instance_profile_arn
}

output "redpanda_node_group_instance_profile_arn" {
  value = module.redpanda-byovpc.redpanda_node_group_instance_profile_arn
}

output "k8s_cluster_role_arn" {
  value = module.redpanda-byovpc.k8s_cluster_role_arn
}


