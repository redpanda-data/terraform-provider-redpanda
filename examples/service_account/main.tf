provider "redpanda" {}

resource "redpanda_service_account" "test" {
  name        = var.service_account_name
  description = var.service_account_description
}

output "client_id" {
  value = redpanda_service_account.test.auth0_client_credentials.client_id
}
