data "redpanda_cloud_provider_access" "example" {
  id = var.cloud_provider_access_id
}

output "role_arn" {
  value = data.redpanda_cloud_provider_access.example.aws.role_arn
}
