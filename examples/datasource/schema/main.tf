provider "redpanda" {}

# Bearer auth (default): omit username and password
data "redpanda_schema" "example" {
  cluster_id = var.cluster_id
  subject    = var.subject
}

# Basic auth: supply username + password when the cluster requires SASL credentials
# data "redpanda_schema" "example_basic" {
#   cluster_id = var.cluster_id
#   subject    = var.subject
#   username   = var.username
#   password   = var.password
# }
