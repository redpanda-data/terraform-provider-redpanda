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
  source  = "redpanda-data/redpanda-byovpc/gcp"
  service_project_id        = var.project_id
  region            = var.region
  unique_identifier = var.unique_identifier
  force_destroy_mgmt_bucket = var.environment == "dev" ? true : false
  force_destroy_cloud_storage_bucket =  var.environment == "dev" ? true : false
  network_project_id = var.project_id
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