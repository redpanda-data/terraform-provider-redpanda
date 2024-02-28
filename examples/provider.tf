terraform {
  required_providers {
    redpanda = {
      source  = "redpanda-data/redpanda"
      version = "~> 0.2.4"
    }
  }
}

provider "redpanda" {
  client_id      = "your_client_id"
  client_secret  = "your_client_secret"
  cloud_provider = "aws"
  region         = "us-west-2"
  zones          = ["us-west-2a", "us-west-2b"]
}