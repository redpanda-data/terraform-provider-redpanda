provider "redpanda" {}


# Redpanda resource group
resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

# Create Redpanda network with customer managed resources
resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "gcp"
  region            = var.region
  cluster_type      = "byoc"

  customer_managed_resources = {
    gcp = {
      network_name = var.vpc_network_name
      network_project_id = var.network_project_id
      management_bucket = {
        name = var.management_bucket_name
      }
    }
  }
}

# Create Redpanda cluster with customer managed resources
resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = "gcp"
  region            = var.region
  cluster_type      = "byoc"
  connection_type   = "private"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true

  tags = {
    "environment" = var.environment
    "managed-by"  = "terraform"
  }

  # Customer managed resources for GCP
  customer_managed_resources = {
    gcp = {
      subnet = {
        name = var.subnet_name
        secondary_ipv4_range_pods = {
          name = var.secondary_ipv4_range_pods_name
        }
        secondary_ipv4_range_services = {
          name = var.secondary_ipv4_range_services_name
        }
        k8s_master_ipv4_range = var.k8s_master_ipv4_range
      }
      agent_service_account = {
        email = var.agent_service_account_email
      }
      console_service_account = {
        email = var.console_service_account_email
      }
      connector_service_account = {
        email = var.connector_service_account_email
      }
      redpanda_cluster_service_account = {
        email = var.redpanda_cluster_service_account_email
      }
      gke_service_account = {
        email = var.gke_service_account_email
      }
      tiered_storage_bucket = {
        name = var.tiered_storage_bucket_name
      }
    }
  }
}

# # Create Kafka user for the cluster
# resource "redpanda_user" "test" {
#   name            = var.user_name
#   password        = var.user_pw
#   mechanism       = var.mechanism
#   cluster_api_url = redpanda_cluster.test.cluster_api_url
# }
#
# # Create Kafka topic in the cluster
# resource "redpanda_topic" "test" {
#   name               = var.topic_name
#   partition_count    = var.partition_count
#   replication_factor = var.replication_factor
#   cluster_api_url    = redpanda_cluster.test.cluster_api_url
#   allow_deletion     = true
# }
#
# # Set ACL for the user on the topic
# resource "redpanda_acl" "test" {
#   resource_type         = "TOPIC"
#   resource_name         = redpanda_topic.test.name
#   resource_pattern_type = "LITERAL"
#   principal             = "User:${redpanda_user.test.name}"
#   host                  = "*"
#   operation             = "READ"
#   permission_type       = "ALLOW"
#   cluster_api_url       = redpanda_cluster.test.cluster_api_url
# }

# Variables
variable "region" {
  description = "GCP region for resources"
  type        = string
  default     = "us-central1"
}

variable "resource_group_name" {
  description = "Redpanda resource group name"
  type        = string
  default     = "testname"
}

variable "network_name" {
  description = "Name for the Redpanda network"
  type        = string
  default     = "testname"
}

variable "cluster_name" {
  description = "Name for the Redpanda cluster"
  type        = string
  default     = "testname"
}

variable "throughput_tier" {
  description = "Throughput tier for the Redpanda cluster"
  type        = string
  default     = "tier-1-gcp-um4g"
}

variable "zones" {
  description = "GCP zones for the Redpanda cluster"
  type        = list(string)
  default     = ["us-central1-a", "us-central1-b", "us-central1-c"]
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}
#
# variable "user_name" {
#   description = "Kafka user name"
#   type        = string
#   default     = "test-username"
# }
#
# variable "user_pw" {
#   description = "Kafka user password"
#   type        = string
#   sensitive   = true
#   default     = "password"
# }
#
# variable "mechanism" {
#   description = "Kafka authentication mechanism"
#   type        = string
#   default     = "scram-sha-256"
# }
#
# variable "topic_name" {
#   description = "Kafka topic name"
#   type        = string
#   default     = "test-topic"
# }
#
# variable "partition_count" {
#   description = "Number of partitions for the Kafka topic"
#   type        = number
#   default     = 3
# }
#
# variable "replication_factor" {
#   description = "Replication factor for the Kafka topic"
#   type        = number
#   default     = 3
# }

# Infrastructure variables that map to customVars
variable "network_project_id" {
  description = "GCP project ID for the network"
  type        = string
}

variable "vpc_network_name" {
  description = "VPC network name"
  type        = string
}

variable "management_bucket_name" {
  description = "Management bucket name"
  type        = string
}

variable "subnet_name" {
  description = "Subnet name"
  type        = string
}

variable "secondary_ipv4_range_pods_name" {
  description = "Secondary IPv4 range name for pods"
  type        = string
}

variable "secondary_ipv4_range_services_name" {
  description = "Secondary IPv4 range name for services"
  type        = string
}

variable "k8s_master_ipv4_range" {
  description = "Kubernetes master IPv4 range"
  type        = string
}

variable "agent_service_account_email" {
  description = "Agent service account email"
  type        = string
}

variable "console_service_account_email" {
  description = "Console service account email"
  type        = string
}

variable "connector_service_account_email" {
  description = "Connector service account email"
  type        = string
}

variable "redpanda_cluster_service_account_email" {
  description = "Redpanda cluster service account email"
  type        = string
}

variable "gke_service_account_email" {
  description = "GKE service account email"
  type        = string
}

variable "tiered_storage_bucket_name" {
  description = "Tiered storage bucket name"
  type        = string
}