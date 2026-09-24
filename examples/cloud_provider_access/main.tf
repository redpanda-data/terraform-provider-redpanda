provider "redpanda" {}

data "redpanda_cloud_provider_access_prerequisites" "test" {
  cloud_provider = "aws"
}

resource "redpanda_cloud_provider_access" "test" {
  name           = var.cloud_provider_access_name
  cloud_provider = "aws"
  aws = {
    role_arn = var.role_arn
  }

  lifecycle {
    create_before_destroy = true
  }
}

data "redpanda_cloud_provider_access" "test" {
  id = redpanda_cloud_provider_access.test.id
}
