# GitHub Actions のデプロイ(production-deploy.yml)が使う値。GitHub の Environment「production」の変数に設定する。

output "azure_resource_group_name" {
  description = "AZURE_RESOURCE_GROUP"
  value       = azurerm_resource_group.production.name
}

output "aks_cluster_name" {
  description = "AKS_CLUSTER_NAME"
  value       = module.aks.name
}

output "acr_name" {
  description = "ACR_NAME"
  value       = module.acr.name
}

output "acr_login_server" {
  description = "イメージ名の先頭(例: pingucoinprodacr.azurecr.io)"
  value       = module.acr.login_server
}

output "key_vault_name" {
  description = "KEY_VAULT_NAME"
  value       = module.keyvault.name
}

# ---- 参考 ----

output "postgres_fqdn" {
  description = "DB のホスト名(仮想ネットワークの中だけで引ける)。"
  value       = module.postgres.fqdn
}

output "postgres_databases" {
  value = module.postgres.database_names
}

output "azure_blob_endpoint" {
  value = module.storage.blob_endpoint
}

output "azure_storage_public_container" {
  description = "AZURE_STORAGE_PUBLIC_CONTAINER"
  value       = module.storage.public_container
}

output "azure_storage_private_container" {
  description = "AZURE_STORAGE_PRIVATE_CONTAINER"
  value       = module.storage.private_container
}
