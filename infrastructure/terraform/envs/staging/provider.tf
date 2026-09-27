terraform {
  required_version = ">= 1.7.0"

  required_providers {
    supabase = {
      source  = "supabase/supabase"
      version = "~> 1.0"
    }

    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 5.0"
    }
  }
}

# Reads the access token from SUPABASE_ACCESS_TOKEN.
provider "supabase" {}

# Uses the credentials created by `az login`.
provider "azurerm" {
  subscription_id = var.azure_subscription_id

  features {}
}
