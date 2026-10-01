locals {
  tags = {
    environment = "staging"
    application = "PinguCoin"
    managed_by  = "Terraform"
  }
}

resource "azurerm_resource_group" "staging" {
  name     = var.resource_group_name
  location = var.location
  tags     = local.tags
}

resource "azurerm_storage_account" "staging" {
  name                = var.storage_account_name
  resource_group_name = azurerm_resource_group.staging.name
  location            = azurerm_resource_group.staging.location

  account_kind             = "StorageV2"
  account_tier             = "Standard"
  account_replication_type = "LRS"
  access_tier              = "Hot"

  https_traffic_only_enabled      = true
  min_tls_version                 = "TLS1_2"
  public_network_access           = "Enabled"
  shared_access_key_enabled       = true
  allow_nested_items_to_be_public = true

  blob_properties {
    cors_rule {
      allowed_origins    = var.cors_allowed_origins
      allowed_methods    = ["DELETE", "GET", "HEAD", "OPTIONS", "PUT"]
      allowed_headers    = ["*"]
      exposed_headers    = ["*"]
      max_age_in_seconds = 3600
    }
  }

  tags = local.tags
}

resource "azurerm_storage_container" "public" {
  name                  = "pingue-public"
  storage_account_id    = azurerm_storage_account.staging.id
  container_access_type = "blob"
}

resource "azurerm_storage_container" "private" {
  name                  = "pingue"
  storage_account_id    = azurerm_storage_account.staging.id
  container_access_type = "private"
}
