---
page_title: "Provider: Redpanda"
description: |-
  The Redpanda Data Terraform provider is used to manage Redpanda Dedicated and Cloud clusters and Kafka resources within them. To connect to a Redpanda Cloud cluster, a client_id and client_secret are required.
---

# Redpanda Provider

The Redpanda provider is designed for managing Redpanda clusters and Kafka resources in Redpanda Dedicated and Cloud environments. It supports the provisioning, management, and configuration of clusters and Kafka resources, facilitating seamless integration into Terraform workflows.

It is also able to provide management of Kafka resources (topics, ACLs, and more) within Redpanda clusters deployed outside of Redpanda Cloud.

This provider requires a `client_id` and `client_secret` for authentication with Redpanda Cloud services, enabling users to securely manage their Redpanda resources. You can get these by creating an account in [Redpanda Cloud](https://cloudv2.redpanda.com/home) and then [creating a client in the ](https://cloudv2.redpanda.com/clients).

## Example Usage

Here's a basic example of how to use the provider:

```hcl
provider "redpanda" {
  client_id      = "your_client_id"
  client_secret  = "your_client_secret"
  cloud_provider = "aws"
  region         = "us-west-2"
  zones          = ["us-west-2a", "us-west-2b"]
}
```