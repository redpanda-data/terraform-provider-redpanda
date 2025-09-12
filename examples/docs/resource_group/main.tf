provider "redpanda" {}

resource "redpanda_resource_group" "example" {
  name = "example-resource-group"
}