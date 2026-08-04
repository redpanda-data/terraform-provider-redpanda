provider "google" {
  project     = var.project_id
  region      = var.region
  credentials = base64decode(var.google_credentials_base64)
}


# Use the Redpanda GCP BYOVPC module
# unique_identifier is required and must be 1-9 characters. The module derives
# local.postfix from it and builds the operator service account as
# "rp-op${local.postfix}" — an empty postfix leaves "rp-op", 5 characters, under
# GCP's 6-character account_id minimum. The upper bound comes from the longest
# name, "redpanda-connect-api-<id>", against the 30-character maximum.
variable "unique_identifier" {
  type = string

  validation {
    condition     = length(var.unique_identifier) >= 1 && length(var.unique_identifier) <= 9
    error_message = "unique_identifier must be 1-9 characters; see the comment above."
  }
}
module "redpanda_gcp" {
  // Git-ref pin, mirroring the AWS stack: the latest registry release (1.0.3)
  // predates the module's rpsql support (enable_redpanda_sql, #16/#17), the
  // BigLake IAM grants (#18) and the operator workload-identity fix (#19). An
  // unpinned source would also silently adopt new releases in CI.
  // Re-pin once a registry release containing those ships.
  source                             = "git::https://github.com/redpanda-data/terraform-gcp-redpanda-byovpc.git?ref=6b660cb4bb0668a1f6979a5c7cb35aca4d9943bb"
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
