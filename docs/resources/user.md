---
page_title: "redpanda_user Resource - terraform-provider-redpanda"
subcategory: ""
description: |-
  User is a user that can be created in Redpanda
---

# redpanda_user (Resource)

User is a user that can be created in Redpanda

Creates a topic in a Redpanda Cluster

## Usage

```terraform
provider "redpanda" {}

resource "redpanda_namespace" "test" {
  name = var.namespace_name
}

resource "redpanda_network" "test" {
  name           = var.network_name
  namespace_id   = redpanda_namespace.test.id
  cloud_provider = var.cloud_provider
  region         = var.region
  cluster_type   = "dedicated"
  cidr_block     = "10.0.0.0/20"
}


resource "redpanda_cluster" "test" {
  name            = var.cluster_name
  namespace_id    = redpanda_namespace.test.id
  network_id      = redpanda_network.test.id
  cloud_provider  = var.cloud_provider
  region          = var.region
  cluster_type    = "dedicated"
  connection_type = "public"
  throughput_tier = var.throughput_tier
  zones           = var.zones
  allow_deletion  = true
  tags            = {
    // not actually used as API does not consume it yet but we keep it in state for when it does
    "key" = "value"
  }
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}


resource "redpanda_acl" "test" {
  resource_type         = "topic"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "literal"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}

variable "namespace_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-1"
}

variable "zones" {
  default = ["use1-az2", "use1-az4", "use1-az6"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-arm"
}

variable "user_name" {
  default = "test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}
```

## Security Considerations

We recommend storing user passwords in a secret store and only passing them in at runtime. They are marked sensitive and will not be shown in plan or apply output.

However it is worth remembering that [marking a field sensitive](https://developer.hashicorp.com/terraform/plugin/best-practices/sensitive-state) does not obscure it or encrypt it in state and that terraform offers limited support on this matter.

Be cognizant of the security implications of storing sensitive data in state and consider using a secure state backend for your state files.

## Limitations

We are not currently able to support user creation in self hosted clusters. This is an area of active development so expect that to change soon.