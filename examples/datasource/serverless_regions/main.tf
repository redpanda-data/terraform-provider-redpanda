provider "redpanda" {}

data "redpanda_serverless_regions" "aws" {
  cloud_provider = "aws"
}

data "redpanda_serverless_regions" "gcp" {
  cloud_provider = "gcp"
}

output "aws_serverless_regions" {
  value = data.redpanda_serverless_regions.aws.serverless_regions
}

output "gcp_serverless_regions" {
  value = data.redpanda_serverless_regions.gcp.serverless_regions
}
