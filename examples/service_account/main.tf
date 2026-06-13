provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.service_account_name
}

resource "redpanda_service_account" "test" {
  name        = var.service_account_name
  description = var.service_account_description
  role_bindings = [
    {
      role_name = var.role_name
      scope = {
        resource_type = "RESOURCE_GROUP"
        resource_id   = redpanda_resource_group.test.id
      }
    },
  ]
}

output "client_id" {
  value = redpanda_service_account.test.auth0_client_credentials.client_id
}
