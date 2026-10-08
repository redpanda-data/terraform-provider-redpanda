data "redpanda_cloud_provider_access_prerequisites" "aws" {
  cloud_provider = "aws"
}

output "trust_principal_arn" {
  value = data.redpanda_cloud_provider_access_prerequisites.aws.aws.principal_arn
}

output "trust_external_id" {
  value = data.redpanda_cloud_provider_access_prerequisites.aws.aws.external_id
}
