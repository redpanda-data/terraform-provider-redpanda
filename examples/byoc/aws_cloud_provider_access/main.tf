provider "redpanda" {}

provider "aws" {
  region = var.region
}

data "redpanda_cloud_provider_access_prerequisites" "aws" {
  cloud_provider = "aws"
}

# The role Redpanda assumes to provision the network, agent and cluster in
# this account. Only the principal and external ID from the prerequisites
# data source can assume it.
# name_prefix rather than name: create_before_destroy on the access below
# extends to this role, and a replacement role must not collide by name.
resource "aws_iam_role" "redpanda" {
  name_prefix = var.role_name_prefix
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

resource "aws_iam_role_policy_attachment" "redpanda" {
  role       = aws_iam_role.redpanda.name
  policy_arn = var.permissions_policy_arn
}

resource "redpanda_cloud_provider_access" "aws" {
  # Named after the role, which name_prefix makes unique, so a replacement
  # role yields a new access name and create_before_destroy never collides.
  name           = aws_iam_role.redpanda.name
  cloud_provider = "aws"
  aws = {
    role_arn = aws_iam_role.redpanda.arn
  }

  # Redpanda assumes the role to destroy the cluster and network, so the
  # role's permissions must outlive both.
  depends_on = [aws_iam_role_policy_attachment.redpanda]

  # Redpanda refuses to delete an access a network still uses. With this,
  # changing the role (and the name, which is unique) creates the new access,
  # re-points the network in place, and deletes the old access last.
  lifecycle {
    create_before_destroy = true
  }
}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "test" {
  name                     = var.network_name
  resource_group_id        = redpanda_resource_group.test.id
  cloud_provider           = "aws"
  region                   = var.region
  cluster_type             = "byoc"
  cidr_block               = "10.0.0.0/20"
  cloud_provider_access_id = redpanda_cloud_provider_access.aws.id
}

resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = redpanda_network.test.cloud_provider
  region            = redpanda_network.test.region
  cluster_type      = redpanda_network.test.cluster_type
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = var.cluster_allow_deletion
}
