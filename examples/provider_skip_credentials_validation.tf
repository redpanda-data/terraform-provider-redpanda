terraform {
  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
  }
}

# One configuration for every workspace. Workspaces that enable Redpanda
# supply REDPANDA_CLIENT_ID and REDPANDA_CLIENT_SECRET; the others configure
# the provider without credentials and never use it.
provider "redpanda" {
  skip_credentials_validation = true
}

variable "redpanda_enabled" {
  type    = bool
  default = false
}

resource "redpanda_resource_group" "shared" {
  count = var.redpanda_enabled ? 1 : 0
  name  = "shared"
}
