provider "redpanda" {}

resource "redpanda_resource_group" "example" {
  name = "example-resource-group"
}

resource "redpanda_service_account" "example" {
  name        = "ci-bot"
  description = "Service account used by CI workloads."
  role_bindings = [
    {
      role_name = "Reader"
      scope = {
        resource_type = "RESOURCE_GROUP"
        resource_id   = redpanda_resource_group.example.id
      }
    },
  ]
}

output "client_id" {
  value = redpanda_service_account.example.auth0_client_credentials.client_id
}

output "client_secret" {
  value     = redpanda_service_account.example.auth0_client_credentials.client_secret
  sensitive = true
}
