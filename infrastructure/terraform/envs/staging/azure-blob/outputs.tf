output "resource_group_name" {
  value = azurerm_resource_group.staging.name
}

output "storage_account_name" {
  value = azurerm_storage_account.staging.name
}

output "blob_endpoint" {
  value = azurerm_storage_account.staging.primary_blob_endpoint
}

output "public_container" {
  value = azurerm_storage_container.public.name
}

output "private_container" {
  value = azurerm_storage_container.private.name
}

output "connection_string" {
  value     = azurerm_storage_account.staging.primary_connection_string
  sensitive = true
}
