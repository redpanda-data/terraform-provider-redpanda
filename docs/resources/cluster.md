---
page_title: "redpanda_cluster Resource - terraform-provider-redpanda"
subcategory: ""
description: |-
  
---

# redpanda_cluster (Resource)



Enables the provisioning and management of Redpanda clusters on AWS and GCP. A cluster must always have a network and namespace and there is not much point to creating either a network or a namespace separately from a cluster.

## Usage

### On AWS

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
```

### On GCP

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
    "key" = "value"
  }
}
variable "cluster_name" {
  default = ""
}
variable "namespace_name" {
  default = ""
}
variable "network_name" {
  default = ""
}

variable "region" {
  default = "us-central1"
}

variable "zones" {
  default = ["us-central1-a", "us-central1-b", "us-central1-c"]
}

variable "cloud_provider" {
  default = "gcp"
}

variable "throughput_tier" {
  default = "tier-1-gcp-um4g"
}
```

## Limitations

We are not currently able to support the provisioning of "BYOC" clusters using this provider. A workaround is available

 * First use [RPK](https://docs.redpanda.com/current/deploy/deployment-option/cloud/create-byoc-cluster-aws/) to provision the cluster
 * Then use the provider's redpanda_cluster data source to reference the cluster for use in other resources.

### Example Usage of a data source BYOC to manage users and ACLs

```terraform
provider "redpanda" {}

variable "cluster_id" {
  default = ""
}
data "redpanda_cluster" "test" {
  id = var.cluster_id
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = data.redpanda_cluster.test.cluster_api_url
}

resource "redpanda_acl" "test" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALTER"
  permission_type       = "ALLOW"
  cluster_api_url       = data.redpanda_cluster.test.cluster_api_url
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