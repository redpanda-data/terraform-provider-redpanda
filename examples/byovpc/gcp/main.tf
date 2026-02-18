provider "google" {
  project     = var.project_id
  region      = var.region
  credentials = base64decode(var.google_credentials_base64)
}

provider "redpanda" {}

# Use the Redpanda GCP BYOVPC module
module "redpanda_gcp" {
  source  = "redpanda-data/redpanda-byovpc/gcp"
  service_project_id        = var.project_id
  region            = var.region
  unique_identifier = var.environment
  force_destroy_mgmt_bucket = var.environment == "dev" ? true : false
  force_destroy_cloud_storage_bucket =  var.environment == "dev" ? true : false
  network_project_id = var.project_id
}

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
      network_name = module.redpanda_gcp.network_name
      network_project_id = var.project_id
      management_bucket = {
        name = module.redpanda_gcp.management_bucket_name
      }
    }
  }
  depends_on = [module.redpanda_gcp]
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
        name = module.redpanda_gcp.subnet_name
        secondary_ipv4_range_pods = {
          name = module.redpanda_gcp.secondary_ipv4_range_pods_name
        }
        secondary_ipv4_range_services = {
          name = module.redpanda_gcp.secondary_ipv4_range_services_name
        }
        k8s_master_ipv4_range = module.redpanda_gcp.k8s_master_ipv4_range
      }
      agent_service_account = {
        email = module.redpanda_gcp.agent_service_account_email
      }
      console_service_account = {
        email = module.redpanda_gcp.console_service_account_email
      }
      connector_service_account = {
        email = module.redpanda_gcp.connector_service_account_email
      }
      redpanda_cluster_service_account = {
        email = module.redpanda_gcp.redpanda_cluster_service_account_email
      }
      gke_service_account = {
        email = module.redpanda_gcp.gke_service_account_email
      }
      tiered_storage_bucket = {
        name = module.redpanda_gcp.tiered_storage_bucket_name
      }
    }
  }
  depends_on = [redpanda_network.test]
}

# Create Kafka user for the cluster
resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = var.user_allow_deletion
}

# Create Kafka topic in the cluster
resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}

# Set ACL for the user on the topic
resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}
