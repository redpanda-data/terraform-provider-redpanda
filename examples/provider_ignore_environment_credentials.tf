terraform {
  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
  }
}

# No credentials, and the REDPANDA_CLIENT_ID, REDPANDA_CLIENT_SECRET, and
# REDPANDA_ACCESS_TOKEN environment variables are ignored: any resource that
# falls back to this default configuration fails instead of landing in the
# wrong organization.
provider "redpanda" {
  ignore_environment_credentials = true
}

provider "redpanda" {
  alias         = "non_prod"
  client_id     = var.non_prod_client_id
  client_secret = var.non_prod_client_secret
}

provider "redpanda" {
  alias         = "prod"
  client_id     = var.prod_client_id
  client_secret = var.prod_client_secret
}

variable "non_prod_client_id" {
  type = string
}

variable "non_prod_client_secret" {
  type      = string
  sensitive = true
}

variable "prod_client_id" {
  type = string
}

variable "prod_client_secret" {
  type      = string
  sensitive = true
}

resource "redpanda_resource_group" "staging" {
  provider = redpanda.non_prod
  name     = "staging"
}

resource "redpanda_resource_group" "production" {
  provider = redpanda.prod
  name     = "production"
}
