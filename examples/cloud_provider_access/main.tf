resource "redpanda_cloud_provider_access" "example" {
  name           = "prod-aws-account"
  cloud_provider = "aws"

  aws = {
    role_arn = var.cross_account_role_arn
  }
}

output "cloud_provider_access_id" {
  value = redpanda_cloud_provider_access.example.id
}

output "external_id" {
  description = "Include this in the IAM role trust policy's sts:ExternalId condition."
  value       = redpanda_cloud_provider_access.example.aws.external_id
}
