terraform {
  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
    # The registry release of the byovnet module targets the azurerm 3.x
    # resource schema; its main branch has moved to 4.x but is unreleased.
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.100"
    }
  }
}
