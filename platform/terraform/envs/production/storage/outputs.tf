output "storage_account_name" {
  value = azurerm_storage_account.this.name
}

output "blob_endpoint" {
  value = azurerm_storage_account.this.primary_blob_endpoint
}

output "public_container" {
  value = azurerm_storage_container.public.name
}

output "private_container" {
  value = azurerm_storage_container.private.name
}

output "connection_string" {
  value     = azurerm_storage_account.this.primary_connection_string
  sensitive = true
}
