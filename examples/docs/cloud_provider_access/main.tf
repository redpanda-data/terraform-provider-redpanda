provider "redpanda" {}

data "redpanda_cloud_provider_access_prerequisites" "aws" {
  cloud_provider = "aws"
}

resource "aws_iam_role" "redpanda" {
  name_prefix = "redpanda-cross-account-"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { AWS = data.redpanda_cloud_provider_access_prerequisites.aws.aws.principal_arn }
      Condition = {
        StringEquals = { "sts:ExternalId" = data.redpanda_cloud_provider_access_prerequisites.aws.aws.external_id }
      }
    }]
  })
}

resource "redpanda_cloud_provider_access" "example" {
  name           = aws_iam_role.redpanda.name
  cloud_provider = "aws"
  aws = {
    role_arn = aws_iam_role.redpanda.arn
  }

  lifecycle {
    create_before_destroy = true
  }
}

output "cloud_provider_access_id" {
  value = redpanda_cloud_provider_access.example.id
}
