terraform {
  required_version = ">= 1.11"

  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
    random = {
      source = "hashicorp/random"
    }
  }
}
