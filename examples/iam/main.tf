provider "redpanda" {}

resource "redpanda_group" "test" {
  name        = var.group_name
  description = var.group_description
}

resource "redpanda_service_account" "test" {
  name        = var.service_account_name
  description = var.service_account_description
}

resource "redpanda_role_binding" "test" {
  role_name  = var.role_name
  account_id = redpanda_service_account.test.id
}
