provider "redpanda" {}

resource "redpanda_service_account" "example" {
  name        = "ci-bot"
  description = "Service account used by CI workloads."
}

output "client_id" {
  value = redpanda_service_account.example.auth0_client_credentials.client_id
}

output "client_secret" {
  value     = redpanda_service_account.example.auth0_client_credentials.client_secret
  sensitive = true
}
