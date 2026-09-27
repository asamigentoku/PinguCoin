output "project_ref" {
  value = module.supabase.project_ref
}

output "db_host" {
  value = module.supabase.db_host
}

output "db_name" {
  value = "postgres"
}

output "db_direct_port" {
  value = 5432
}

output "db_sslmode" {
  value = "require"
}

output "azure_resource_group_name" {
  value = module.azure_blob.resource_group_name
}

output "azure_storage_account_name" {
  value = module.azure_blob.storage_account_name
}

output "azure_blob_endpoint" {
  value = module.azure_blob.blob_endpoint
}

output "azure_storage_public_container" {
  value = module.azure_blob.public_container
}

output "azure_storage_private_container" {
  value = module.azure_blob.private_container
}

output "azure_storage_connection_string" {
  description = "Set this as AZURE_STORAGE_CONNECTION_STRING in the orcan-api Vercel project."
  value       = module.azure_blob.connection_string
  sensitive   = true
}
