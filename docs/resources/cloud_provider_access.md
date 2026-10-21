---
page_title: "redpanda_cloud_provider_access Resource - terraform-provider-redpanda"
subcategory: ""
description: |-
  Manages a Redpanda Cloud cloud provider access
---

# redpanda_cloud_provider_access (Resource)

Manages a Redpanda Cloud cloud provider access

~> **Preview:** cloud provider access is a preview feature of Redpanda Cloud. Your organization must have cross-account BYOC enabled; contact Redpanda support to enable it.

Registers an IAM role in your AWS account that Redpanda assumes to provision BYOC infrastructure. Set the access's `id` as `cloud_provider_access_id` on a BYOC `redpanda_network`, and Redpanda creates the network, the BYOC agent and the clusters on it with that role. You don't need AWS credentials for the provider or a `rpk cloud byoc aws apply` run. One access can back several networks in the same AWS account. Only AWS is supported.

## Trust Policy

Creating the access records the role without assuming it, so a missing or wrong trust policy only surfaces when a network is provisioned. Build the trust policy from the `redpanda_cloud_provider_access_prerequisites` data source: allow its `aws.principal_arn` to call `sts:AssumeRole`, and require its `aws.external_id` in an `sts:ExternalId` condition. The same external ID is exposed on the access as `aws.external_id`.

## Permissions Policy

The role also needs the permissions to create and destroy the BYOC infrastructure. Five resource-scoped IAM policy documents grant them, in [`examples/byoc/aws_cloud_provider_access/policies/`](https://github.com/redpanda-data/terraform-provider-redpanda/tree/main/examples/byoc/aws_cloud_provider_access/policies). Create each one as a customer managed policy and attach it to the role:

| Policy | Required | Grants |
|---|---|---|
| `redpanda-byoc-common.json` | Yes | Read-only discovery, the management bucket `rp-<account-id>-*-mgmt`, and the legacy Terraform lock table. |
| `redpanda-byoc-network.json` | Yes | The VPC, subnets, NAT gateways, internet gateway, route tables, VPC endpoints, and VPC flow logs with their bucket. |
| `redpanda-byoc-agent.json` | Yes | The agent launch template, security group, and Auto Scaling group; the agent IAM role and instance profile; the IAM policies of the agent and the cluster; and the tiered storage bucket, including deletion of its objects when the cluster is destroyed. |
| `redpanda-byoc-peering.json` | No | VPC peering connections. Attach it only if you use VPC peering. |
| `redpanda-byoc-transit-gateway.json` | No | Transit gateway VPC attachments, their route table associations and propagations, the endpoint security group, and the association of the VPC with private hosted zones. Attach it only if you use a transit gateway. |

Keep the policies separate. The agent policy is near the IAM size limit for a managed policy, so you cannot merge it with another policy. Load the documents with `file()`, not `templatefile()`: the common policy contains the IAM policy variable `${aws:PrincipalAccount}`, which `templatefile()` tries to interpolate. If you inline it in HCL, escape the variable as `$${aws:PrincipalAccount}`. The [AWS BYOC example](https://github.com/redpanda-data/terraform-provider-redpanda/tree/main/examples/byoc/aws_cloud_provider_access) creates and attaches the policies, with the optional ones behind the `enable_vpc_peering` and `enable_transit_gateway` variables.

The agent policy lets the role create and edit IAM policies whose names start with `redpanda-agent-`, `redpanda-cloud-storage-manager-`, or `redpanda-glue-manager-`. It also lets the role create IAM roles whose names start with `redpanda-agent-`, attach `redpanda-agent-*` policies to them, and pass them to EC2. It does not restrict the content of those policies, so you trust the Redpanda principal with the permissions it grants to them. A permissions boundary on the cross-account role does not limit the roles it creates. Only a service control policy sets a hard limit. Do not give the role or its policies a name with one of these prefixes, because the role could then edit its own policies. The example validates `role_name_prefix` for this.

This role does not manage the resources that the BYOC agent runs, such as the EKS cluster, load balancers, DNS records, and the PrivateLink endpoint service of the cluster. The agent manages them with its own instance profile.

Make the access depend on the policy attachments, as in the AWS BYOC example, so that Terraform creates the permissions before the network and deletes them after it. Keep an optional policy attached while a network still uses it: if you detach the peering or transit gateway policy first, Redpanda cannot delete the peering connections or attachments.

## Rotating the Role

The access has no update API, so changing any argument replaces it. Networks are never replaced for this: a network's `cloud_provider_access_id` changes in place to another access for the same AWS account. Adding an access to a network created without one, or removing it, is refused by Redpanda Cloud; the provider warns at plan rather than replacing the network, because replacing a network replaces every cluster on it. To move a network onto a cloud provider access, replace it deliberately, for example with `terraform apply -replace`.

Redpanda refuses to delete an access while a network still uses it, and by default Terraform deletes a replaced or removed resource before updating the resources that reference it. Set `lifecycle { create_before_destroy = true }` on the access, as in the example below, so that Terraform instead:

1. Creates the new access.
2. Re-points every network that references it, in place.
3. Deletes the old access last.

Access names are unique within an organization, so change the `name` together with the `role_arn`; the example below derives the name from the IAM role, whose `name_prefix` makes each replacement role's name unique. The same applies when you rotate by adding a second access block and moving the networks to it: the flag must already be recorded on the old access, so set it when you first create the access. Without it, the apply fails when Terraform deletes the old access, and nothing is changed.

## Deletion

Terraform destroys the networks that reference an access before the access itself. Keep the IAM role and its permissions until the clusters and networks using the access are destroyed, because Redpanda assumes the role to tear them down.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `aws` (Attributes) AWS cross-account access configuration, used when `cloud_provider` is `aws`. (see [below for nested schema](#nestedatt--aws))
- `cloud_provider` (String) Cloud provider type. Only `aws` is supported. If the value of this attribute changes, Terraform will destroy and recreate the resource.
- `name` (String) Human-readable name for this cloud provider access. Unique within the organization. Length must be between 1 and 128. Must match pattern `^[A-Za-z0-9-_: ]+$`. If the value of this attribute changes, Terraform will destroy and recreate the resource.

### Read-Only

- `id` (String) Unique identifier of the resource. Must match pattern `^[a-v0-9]{20}$`.
- `state` (String) Current state of the cloud provider access.

<a id="nestedatt--aws"></a>
### Nested Schema for `aws`

Required:

- `role_arn` (String) Full ARN of the IAM role Redpanda assumes in your AWS account, in the commercial `aws` partition. The role's trust policy must allow the principal and external ID returned by the `redpanda_cloud_provider_access_prerequisites` data source. Must match pattern `^arn:aws:iam::\d{12}:role/.+$`. If the value of this attribute changes, Terraform will destroy and recreate the resource.

Read-Only:

- `external_id` (String) External ID for STS AssumeRole confused deputy protection, set by the server to the organization ID. Include it in the IAM role trust policy's `sts:ExternalId` condition.

## Example Usage

The example below shows the trust policy only. Attach the permissions policies as in the AWS BYOC example.

```terraform
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
```

## Import

```shell
terraform import redpanda_cloud_provider_access.example cloudProviderAccessId
```

Where `cloudProviderAccessId` is the 20-character identifier assigned by Redpanda Cloud.

## API Reference

For more information, see the [Redpanda Cloud Control Plane API documentation](https://docs.redpanda.com/api/cloud-controlplane-api/).
