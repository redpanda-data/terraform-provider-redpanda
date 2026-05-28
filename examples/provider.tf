terraform {
  required_providers {
    redpanda = {
      source  = "redpanda-data/redpanda"
      version = "~> 1.9"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "redpanda" {
  client_id     = "your_client_id"
  client_secret = "your_client_secret"
}

