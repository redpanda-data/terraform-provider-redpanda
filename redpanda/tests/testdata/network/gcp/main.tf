provider "google" {
  project     = var.project_id
  region      = var.region
  credentials = base64decode(var.google_credentials_base64)
}


# Use the Redpanda GCP BYOVPC module
variable "unique_identifier" {
  default = ""
}
module "redpanda_gcp" {
  source                             = "redpanda-data/redpanda-byovpc/gcp"
  service_project_id                 = var.project_id
  region                             = var.region
  unique_identifier                  = var.unique_identifier
  force_destroy_mgmt_bucket          = var.environment == "dev" ? true : false
  force_destroy_cloud_storage_bucket = var.environment == "dev" ? true : false
  network_project_id                 = var.project_id
}

# Variables
variable "project_id" {
  description = "The Google Cloud project ID"
  type        = string
}

variable "google_credentials_base64" {
  description = "Base64 encoded Google Cloud credentials"
  type        = string
}

variable "region" {
  description = "GCP region for resources"
  type        = string
  default     = "us-central1"
}

variable "environment" {
  default = "dev"
}

# Mirrors the AWS producer: the outputs the acceptance test needs, written to a
# JSON file the task reads with jq.
locals {
  gcp_outputs = {
    network_name                           = module.redpanda_gcp.network_name
    network_project_id                     = module.redpanda_gcp.network_project_id
    subnet_name                            = module.redpanda_gcp.subnet_name
    secondary_ipv4_range_pods_name         = module.redpanda_gcp.secondary_ipv4_range_pods_name
    secondary_ipv4_range_services_name     = module.redpanda_gcp.secondary_ipv4_range_services_name
    k8s_master_ipv4_range                  = module.redpanda_gcp.k8s_master_ipv4_range
    management_bucket_name                 = module.redpanda_gcp.management_bucket_name
    tiered_storage_bucket_name             = module.redpanda_gcp.tiered_storage_bucket_name
    agent_service_account_email            = module.redpanda_gcp.agent_service_account_email
    console_service_account_email          = module.redpanda_gcp.console_service_account_email
    connector_service_account_email        = module.redpanda_gcp.connector_service_account_email
    redpanda_cluster_service_account_email = module.redpanda_gcp.redpanda_cluster_service_account_email
    gke_service_account_email              = module.redpanda_gcp.gke_service_account_email
  }
}

resource "local_file" "inputs_json" {
  content  = jsonencode(local.gcp_outputs)
  filename = "${path.module}/inputs.json"
}

output "gcp_outputs" {
  value = local.gcp_outputs
}
