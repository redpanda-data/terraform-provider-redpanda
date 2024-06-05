terraform {
  required_providers {
    redpanda = {
      source  = "redpanda-data/redpanda"
      version = "~> 0.2.4"
    }
  }
}

provider "redpanda" {
  client_id     = "your_client_id"
  client_secret = "your_client_secret"
}