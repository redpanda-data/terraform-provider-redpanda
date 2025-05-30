---
page_title: "redpanda_cluster Data Source - terraform-provider-redpanda"
subcategory: ""
description: |-
  Data source for a Redpanda Cloud cluster
---

# redpanda_cluster (Data Source)

Data source for a Redpanda Cloud cluster

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) ID of the cluster. ID is an output from the Create Cluster endpoint and cannot be set by the caller.

### Read-Only

- `allow_deletion` (Boolean) Whether cluster deletion is allowed.
- `aws_private_link` (Attributes) AWS PrivateLink configuration. (see [below for nested schema](#nestedatt--aws_private_link))
- `azure_private_link` (Attributes) Azure Private Link configuration. (see [below for nested schema](#nestedatt--azure_private_link))
- `cloud_provider` (String) Cloud provider where resources are created.
- `cluster_api_url` (String) The URL of the cluster API.
- `cluster_type` (String) Cluster type. Type is immutable and can only be set on cluster creation.
- `connection_type` (String) Cluster connection type. Private clusters are not exposed to the internet. For BYOC clusters, Private is best-practice.
- `created_at` (String) Timestamp when the cluster was created.
- `customer_managed_resources` (Attributes) Customer managed resources configuration for the cluster. (see [below for nested schema](#nestedatt--customer_managed_resources))
- `gcp_global_access_enabled` (Boolean) If true, GCP global access is enabled.
- `gcp_private_service_connect` (Attributes) GCP Private Service Connect configuration. (see [below for nested schema](#nestedatt--gcp_private_service_connect))
- `http_proxy` (Attributes) HTTP Proxy properties. (see [below for nested schema](#nestedatt--http_proxy))
- `kafka_api` (Attributes) Cluster's Kafka API properties. (see [below for nested schema](#nestedatt--kafka_api))
- `kafka_connect` (Attributes) Kafka Connect configuration. (see [below for nested schema](#nestedatt--kafka_connect))
- `maintenance_window_config` (Attributes) Maintenance window configuration for the cluster. (see [below for nested schema](#nestedatt--maintenance_window_config))
- `name` (String) Unique name of the cluster.
- `network_id` (String) Network ID where cluster is placed.
- `prometheus` (Attributes) Prometheus metrics endpoint properties. (see [below for nested schema](#nestedatt--prometheus))
- `read_replica_cluster_ids` (List of String) IDs of clusters that can create read-only topics from this cluster.
- `redpanda_console` (Attributes) Redpanda Console properties. (see [below for nested schema](#nestedatt--redpanda_console))
- `redpanda_version` (String) Current Redpanda version of the cluster.
- `region` (String) Cloud provider region.
- `resource_group_id` (String) Resource group ID of the cluster.
- `schema_registry` (Attributes) Schema Registry properties. (see [below for nested schema](#nestedatt--schema_registry))
- `state` (String) Current state of the cluster.
- `state_description` (Attributes) Detailed state description when cluster is in a non-ready state. (see [below for nested schema](#nestedatt--state_description))
- `tags` (Map of String) Tags placed on cloud resources.
- `throughput_tier` (String) Throughput tier of the cluster.
- `zones` (List of String) Zones of the cluster. Must be valid zones within the selected region. If multiple zones are used, the cluster is a multi-AZ cluster.

<a id="nestedatt--aws_private_link"></a>
### Nested Schema for `aws_private_link`

Read-Only:

- `allowed_principals` (List of String) The ARN of the principals that can access the Redpanda AWS PrivateLink Endpoint Service.
- `connect_console` (Boolean) Whether Console is connected via PrivateLink.
- `enabled` (Boolean) Whether AWS PrivateLink is enabled.
- `status` (Attributes) Current status of the PrivateLink configuration. (see [below for nested schema](#nestedatt--aws_private_link--status))

<a id="nestedatt--aws_private_link--status"></a>
### Nested Schema for `aws_private_link.status`

Read-Only:

- `console_port` (Number) Port for Redpanda Console.
- `created_at` (String) When the PrivateLink service was created.
- `deleted_at` (String) When the PrivateLink service was deleted.
- `kafka_api_node_base_port` (Number) Base port for Kafka API nodes.
- `kafka_api_seed_port` (Number) Port for Kafka API seed brokers.
- `redpanda_proxy_node_base_port` (Number) Base port for HTTP proxy nodes.
- `redpanda_proxy_seed_port` (Number) Port for HTTP proxy.
- `schema_registry_seed_port` (Number) Port for Schema Registry.
- `service_id` (String) The PrivateLink service ID.
- `service_name` (String) The PrivateLink service name.
- `service_state` (String) Current state of the PrivateLink service.
- `vpc_endpoint_connections` (Attributes List) List of VPC endpoint connections. (see [below for nested schema](#nestedatt--aws_private_link--status--vpc_endpoint_connections))

<a id="nestedatt--aws_private_link--status--vpc_endpoint_connections"></a>
### Nested Schema for `aws_private_link.status.vpc_endpoint_connections`

Read-Only:

- `connection_id` (String) The connection ID.
- `created_at` (String) When the endpoint connection was created.
- `dns_entries` (Attributes List) DNS entries for the endpoint. (see [below for nested schema](#nestedatt--aws_private_link--status--vpc_endpoint_connections--dns_entries))
- `id` (String) The endpoint connection ID.
- `load_balancer_arns` (List of String) ARNs of associated load balancers.
- `owner` (String) Owner of the endpoint connection.
- `state` (String) State of the endpoint connection.

<a id="nestedatt--aws_private_link--status--vpc_endpoint_connections--dns_entries"></a>
### Nested Schema for `aws_private_link.status.vpc_endpoint_connections.dns_entries`

Read-Only:

- `dns_name` (String) The DNS name.
- `hosted_zone_id` (String) The hosted zone ID.





<a id="nestedatt--azure_private_link"></a>
### Nested Schema for `azure_private_link`

Read-Only:

- `allowed_subscriptions` (List of String) The subscriptions that can access the Redpanda Azure PrivateLink Endpoint Service.
- `connect_console` (Boolean) Whether Console is connected in Redpanda Azure Private Link Service.
- `enabled` (Boolean) Whether Redpanda Azure Private Link Endpoint Service is enabled.
- `status` (Attributes) Current status of the Private Link configuration. (see [below for nested schema](#nestedatt--azure_private_link--status))

<a id="nestedatt--azure_private_link--status"></a>
### Nested Schema for `azure_private_link.status`

Read-Only:

- `approved_subscriptions` (List of String) List of approved Azure subscription IDs.
- `console_port` (Number) Port for Redpanda Console.
- `created_at` (String) When the Private Link service was created.
- `deleted_at` (String) When the Private Link service was deleted.
- `dns_a_record` (String) DNS A record for the service.
- `kafka_api_node_base_port` (Number) Base port for Kafka API nodes.
- `kafka_api_seed_port` (Number) Port for Kafka API seed brokers.
- `private_endpoint_connections` (Attributes List) List of private endpoint connections. (see [below for nested schema](#nestedatt--azure_private_link--status--private_endpoint_connections))
- `redpanda_proxy_node_base_port` (Number) Base port for HTTP proxy nodes.
- `redpanda_proxy_seed_port` (Number) Port for HTTP proxy.
- `schema_registry_seed_port` (Number) Port for Schema Registry.
- `service_id` (String) The Private Link service ID.
- `service_name` (String) The Private Link service name.

<a id="nestedatt--azure_private_link--status--private_endpoint_connections"></a>
### Nested Schema for `azure_private_link.status.private_endpoint_connections`

Read-Only:

- `connection_id` (String) ID of the connection.
- `connection_name` (String) Name of the connection.
- `created_at` (String) When the endpoint connection was created.
- `private_endpoint_id` (String) ID of the private endpoint.
- `private_endpoint_name` (String) Name of the private endpoint.
- `status` (String) Status of the endpoint connection.




<a id="nestedatt--customer_managed_resources"></a>
### Nested Schema for `customer_managed_resources`

Read-Only:

- `aws` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws))
- `gcp` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--gcp))

<a id="nestedatt--customer_managed_resources--aws"></a>
### Nested Schema for `customer_managed_resources.aws`

Read-Only:

- `agent_instance_profile` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--agent_instance_profile))
- `cloud_storage_bucket` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--cloud_storage_bucket))
- `cluster_security_group` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--cluster_security_group))
- `connectors_node_group_instance_profile` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--connectors_node_group_instance_profile))
- `connectors_security_group` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--connectors_security_group))
- `k8s_cluster_role` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--k8s_cluster_role))
- `node_security_group` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--node_security_group))
- `permissions_boundary_policy` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--permissions_boundary_policy))
- `redpanda_agent_security_group` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--redpanda_agent_security_group))
- `redpanda_node_group_instance_profile` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--redpanda_node_group_instance_profile))
- `redpanda_node_group_security_group` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--redpanda_node_group_security_group))
- `utility_node_group_instance_profile` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--utility_node_group_instance_profile))
- `utility_security_group` (Attributes) (see [below for nested schema](#nestedatt--customer_managed_resources--aws--utility_security_group))

<a id="nestedatt--customer_managed_resources--aws--agent_instance_profile"></a>
### Nested Schema for `customer_managed_resources.aws.agent_instance_profile`

Read-Only:

- `arn` (String) ARN for the agent instance profile


<a id="nestedatt--customer_managed_resources--aws--cloud_storage_bucket"></a>
### Nested Schema for `customer_managed_resources.aws.cloud_storage_bucket`

Read-Only:

- `arn` (String) ARN for the cloud storage bucket


<a id="nestedatt--customer_managed_resources--aws--cluster_security_group"></a>
### Nested Schema for `customer_managed_resources.aws.cluster_security_group`

Read-Only:

- `arn` (String) ARN for the cluster security group


<a id="nestedatt--customer_managed_resources--aws--connectors_node_group_instance_profile"></a>
### Nested Schema for `customer_managed_resources.aws.connectors_node_group_instance_profile`

Read-Only:

- `arn` (String) ARN for the connectors node group instance profile


<a id="nestedatt--customer_managed_resources--aws--connectors_security_group"></a>
### Nested Schema for `customer_managed_resources.aws.connectors_security_group`

Read-Only:

- `arn` (String) ARN for the connectors security group


<a id="nestedatt--customer_managed_resources--aws--k8s_cluster_role"></a>
### Nested Schema for `customer_managed_resources.aws.k8s_cluster_role`

Read-Only:

- `arn` (String) ARN for the Kubernetes cluster role


<a id="nestedatt--customer_managed_resources--aws--node_security_group"></a>
### Nested Schema for `customer_managed_resources.aws.node_security_group`

Read-Only:

- `arn` (String) ARN for the node security group


<a id="nestedatt--customer_managed_resources--aws--permissions_boundary_policy"></a>
### Nested Schema for `customer_managed_resources.aws.permissions_boundary_policy`

Read-Only:

- `arn` (String) ARN for the permissions boundary policy


<a id="nestedatt--customer_managed_resources--aws--redpanda_agent_security_group"></a>
### Nested Schema for `customer_managed_resources.aws.redpanda_agent_security_group`

Read-Only:

- `arn` (String) ARN for the redpanda agent security group


<a id="nestedatt--customer_managed_resources--aws--redpanda_node_group_instance_profile"></a>
### Nested Schema for `customer_managed_resources.aws.redpanda_node_group_instance_profile`

Read-Only:

- `arn` (String) ARN for the redpanda node group instance profile


<a id="nestedatt--customer_managed_resources--aws--redpanda_node_group_security_group"></a>
### Nested Schema for `customer_managed_resources.aws.redpanda_node_group_security_group`

Read-Only:

- `arn` (String) ARN for the redpanda node group security group


<a id="nestedatt--customer_managed_resources--aws--utility_node_group_instance_profile"></a>
### Nested Schema for `customer_managed_resources.aws.utility_node_group_instance_profile`

Read-Only:

- `arn` (String) ARN for the utility node group instance profile


<a id="nestedatt--customer_managed_resources--aws--utility_security_group"></a>
### Nested Schema for `customer_managed_resources.aws.utility_security_group`

Read-Only:

- `arn` (String) ARN for the utility security group



<a id="nestedatt--customer_managed_resources--gcp"></a>
### Nested Schema for `customer_managed_resources.gcp`

Read-Only:

- `agent_service_account` (Attributes) GCP service account for the agent. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--agent_service_account))
- `connector_service_account` (Attributes) GCP service account for managed connectors. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--connector_service_account))
- `console_service_account` (Attributes) GCP service account for Redpanda Console. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--console_service_account))
- `gke_service_account` (Attributes) GCP service account for GCP Kubernetes Engine (GKE). (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--gke_service_account))
- `psc_nat_subnet_name` (String) NAT subnet name if GCP Private Service Connect is enabled.
- `redpanda_cluster_service_account` (Attributes) GCP service account for the Redpanda cluster. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--redpanda_cluster_service_account))
- `subnet` (Attributes) GCP subnet where Redpanda cluster is deployed. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--subnet))
- `tiered_storage_bucket` (Attributes) GCP storage bucket for Tiered storage. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--tiered_storage_bucket))

<a id="nestedatt--customer_managed_resources--gcp--agent_service_account"></a>
### Nested Schema for `customer_managed_resources.gcp.agent_service_account`

Read-Only:

- `email` (String) GCP service account email.


<a id="nestedatt--customer_managed_resources--gcp--connector_service_account"></a>
### Nested Schema for `customer_managed_resources.gcp.connector_service_account`

Read-Only:

- `email` (String) GCP service account email.


<a id="nestedatt--customer_managed_resources--gcp--console_service_account"></a>
### Nested Schema for `customer_managed_resources.gcp.console_service_account`

Read-Only:

- `email` (String) GCP service account email.


<a id="nestedatt--customer_managed_resources--gcp--gke_service_account"></a>
### Nested Schema for `customer_managed_resources.gcp.gke_service_account`

Read-Only:

- `email` (String) GCP service account email.


<a id="nestedatt--customer_managed_resources--gcp--redpanda_cluster_service_account"></a>
### Nested Schema for `customer_managed_resources.gcp.redpanda_cluster_service_account`

Read-Only:

- `email` (String) GCP service account email.


<a id="nestedatt--customer_managed_resources--gcp--subnet"></a>
### Nested Schema for `customer_managed_resources.gcp.subnet`

Read-Only:

- `k8s_master_ipv4_range` (String) Kubernetes Master IPv4 range, e.g. 10.0.0.0/24.
- `name` (String) Subnet name.
- `secondary_ipv4_range_pods` (Attributes) Secondary IPv4 range for pods. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--subnet--secondary_ipv4_range_pods))
- `secondary_ipv4_range_services` (Attributes) Secondary IPv4 range for services. (see [below for nested schema](#nestedatt--customer_managed_resources--gcp--subnet--secondary_ipv4_range_services))

<a id="nestedatt--customer_managed_resources--gcp--subnet--secondary_ipv4_range_pods"></a>
### Nested Schema for `customer_managed_resources.gcp.subnet.secondary_ipv4_range_pods`

Read-Only:

- `name` (String) Secondary IPv4 range name for pods.


<a id="nestedatt--customer_managed_resources--gcp--subnet--secondary_ipv4_range_services"></a>
### Nested Schema for `customer_managed_resources.gcp.subnet.secondary_ipv4_range_services`

Read-Only:

- `name` (String) Secondary IPv4 range name for services.



<a id="nestedatt--customer_managed_resources--gcp--tiered_storage_bucket"></a>
### Nested Schema for `customer_managed_resources.gcp.tiered_storage_bucket`

Read-Only:

- `name` (String) GCP storage bucket name.




<a id="nestedatt--gcp_private_service_connect"></a>
### Nested Schema for `gcp_private_service_connect`

Read-Only:

- `consumer_accept_list` (Attributes List) List of consumers that are allowed to connect to Redpanda GCP PSC service attachment. (see [below for nested schema](#nestedatt--gcp_private_service_connect--consumer_accept_list))
- `enabled` (Boolean) Whether Redpanda GCP Private Service Connect is enabled.
- `global_access_enabled` (Boolean) Whether global access is enabled.
- `status` (Attributes) Current status of the Private Service Connect configuration. (see [below for nested schema](#nestedatt--gcp_private_service_connect--status))

<a id="nestedatt--gcp_private_service_connect--consumer_accept_list"></a>
### Nested Schema for `gcp_private_service_connect.consumer_accept_list`

Read-Only:

- `source` (String) Either the GCP project number or its alphanumeric ID.


<a id="nestedatt--gcp_private_service_connect--status"></a>
### Nested Schema for `gcp_private_service_connect.status`

Read-Only:

- `connected_endpoints` (Attributes List) List of connected endpoints. (see [below for nested schema](#nestedatt--gcp_private_service_connect--status--connected_endpoints))
- `created_at` (String) When the Private Service Connect service was created.
- `deleted_at` (String) When the Private Service Connect service was deleted.
- `dns_a_records` (List of String) DNS A records for the service.
- `kafka_api_node_base_port` (Number) Base port for Kafka API nodes.
- `kafka_api_seed_port` (Number) Port for Kafka API seed brokers.
- `redpanda_proxy_node_base_port` (Number) Base port for HTTP proxy nodes.
- `redpanda_proxy_seed_port` (Number) Port for HTTP proxy.
- `schema_registry_seed_port` (Number) Port for Schema Registry.
- `seed_hostname` (String) Hostname for the seed brokers.
- `service_attachment` (String) The service attachment identifier.

<a id="nestedatt--gcp_private_service_connect--status--connected_endpoints"></a>
### Nested Schema for `gcp_private_service_connect.status.connected_endpoints`

Read-Only:

- `connection_id` (String) The connection ID.
- `consumer_network` (String) The consumer network.
- `endpoint` (String) The endpoint address.
- `status` (String) Status of the endpoint connection.




<a id="nestedatt--http_proxy"></a>
### Nested Schema for `http_proxy`

Read-Only:

- `mtls` (Attributes) mTLS configuration. (see [below for nested schema](#nestedatt--http_proxy--mtls))
- `url` (String) The HTTP Proxy URL.

<a id="nestedatt--http_proxy--mtls"></a>
### Nested Schema for `http_proxy.mtls`

Read-Only:

- `ca_certificates_pem` (List of String) CA certificate in PEM format.
- `enabled` (Boolean) Whether mTLS is enabled.
- `principal_mapping_rules` (List of String) Principal mapping rules for mTLS authentication.



<a id="nestedatt--kafka_api"></a>
### Nested Schema for `kafka_api`

Read-Only:

- `mtls` (Attributes) mTLS configuration. (see [below for nested schema](#nestedatt--kafka_api--mtls))
- `seed_brokers` (List of String) List of Kafka broker addresses.

<a id="nestedatt--kafka_api--mtls"></a>
### Nested Schema for `kafka_api.mtls`

Read-Only:

- `ca_certificates_pem` (List of String) CA certificate in PEM format.
- `enabled` (Boolean) Whether mTLS is enabled.
- `principal_mapping_rules` (List of String) Principal mapping rules for mTLS authentication.



<a id="nestedatt--kafka_connect"></a>
### Nested Schema for `kafka_connect`

Read-Only:

- `enabled` (Boolean) Whether Kafka Connect is enabled.


<a id="nestedatt--maintenance_window_config"></a>
### Nested Schema for `maintenance_window_config`

Read-Only:

- `anytime` (Boolean) If true, maintenance can occur at any time.
- `day_hour` (Attributes) (see [below for nested schema](#nestedatt--maintenance_window_config--day_hour))
- `unspecified` (Boolean) If true, maintenance window is unspecified.

<a id="nestedatt--maintenance_window_config--day_hour"></a>
### Nested Schema for `maintenance_window_config.day_hour`

Read-Only:

- `day_of_week` (String) Day of week.
- `hour_of_day` (Number) Hour of day.



<a id="nestedatt--prometheus"></a>
### Nested Schema for `prometheus`

Read-Only:

- `url` (String) The Prometheus metrics endpoint URL.


<a id="nestedatt--redpanda_console"></a>
### Nested Schema for `redpanda_console`

Read-Only:

- `url` (String) The Redpanda Console URL.


<a id="nestedatt--schema_registry"></a>
### Nested Schema for `schema_registry`

Read-Only:

- `mtls` (Attributes) mTLS configuration. (see [below for nested schema](#nestedatt--schema_registry--mtls))
- `url` (String) The Schema Registry URL.

<a id="nestedatt--schema_registry--mtls"></a>
### Nested Schema for `schema_registry.mtls`

Read-Only:

- `ca_certificates_pem` (List of String) CA certificate in PEM format.
- `enabled` (Boolean) Whether mTLS is enabled.
- `principal_mapping_rules` (List of String) Principal mapping rules for mTLS authentication.



<a id="nestedatt--state_description"></a>
### Nested Schema for `state_description`

Read-Only:

- `code` (Number) Error code if cluster is in error state.
- `message` (String) Detailed error message if cluster is in error state.

## Usage

```hcl
data "redpanda_cluster" "example" {
    id = "cluster_id"
}
```

### Example Usage of a data source BYOC to manage users and ACLs

```terraform
provider "redpanda" {}

variable "cluster_id" {
  default = ""
}

data "redpanda_cluster" "test" {
  id = var.cluster_id
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = data.redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
  configuration      = var.topic_config
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

variable "topic_config" {
  default = {
    "cleanup.policy"   = "compact"
    "flush.ms"         = 100
    "compression.type" = "snappy"
  }
}

variable "user_name" {
  default = "data-test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "data-test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}
```

## Limitations

Can only be used with Redpanda Cloud Dedicated and BYOC clusters.