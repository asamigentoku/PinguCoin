locals {
  tags = {
    environment = "production"
    application = "PinguCoin"
    managed_by  = "Terraform"
  }
}

resource "azurerm_resource_group" "production" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.tags
}

module "network" {
  source              = "./network"
  name_prefix         = var.name_prefix
  location            = var.location
  resource_group_name = azurerm_resource_group.production.name
  tags                = local.tags
}

module "monitoring" {
  source              = "./monitoring"
  name_prefix         = var.name_prefix
  location            = var.location
  resource_group_name = azurerm_resource_group.production.name
  tags                = local.tags
}

module "acr" {
  source              = "./acr"
  name                = var.acr_name
  sku                 = var.acr_sku
  location            = var.location
  resource_group_name = azurerm_resource_group.production.name
  tags                = local.tags
}

module "postgres" {
  source                   = "./postgres"
  name                     = var.postgres_server_name
  location                 = var.location
  resource_group_name      = azurerm_resource_group.production.name
  delegated_subnet_id      = module.network.postgres_subnet_id
  private_dns_zone_id      = module.network.postgres_private_dns_zone_id
  private_dns_zone_link_id = module.network.postgres_private_dns_zone_link_id
  sku_name                 = var.postgres_sku_name
  storage_mb               = var.postgres_storage_mb
  high_availability        = var.postgres_high_availability
  backup_retention_days    = var.postgres_backup_retention_days
  geo_redundant_backup     = var.postgres_geo_redundant_backup
  tags                     = local.tags
}

module "storage" {
  source               = "./storage"
  name                 = var.storage_account_name
  location             = var.location
  resource_group_name  = azurerm_resource_group.production.name
  replication_type     = var.storage_replication_type
  cors_allowed_origins = var.storage_cors_allowed_origins
  tags                 = local.tags
}

module "aks" {
  source                     = "./aks"
  name_prefix                = var.name_prefix
  location                   = var.location
  resource_group_name        = azurerm_resource_group.production.name
  subnet_id                  = module.network.aks_subnet_id
  log_analytics_workspace_id = module.monitoring.log_analytics_workspace_id
  kubernetes_version         = var.aks_kubernetes_version
  node_vm_size               = var.aks_node_vm_size
  node_count                 = var.aks_node_count
  sku_tier                   = var.aks_sku_tier
  enable_container_insights  = var.aks_enable_container_insights
  admin_group_object_ids     = var.aks_admin_group_object_ids
  tags                       = local.tags
}

# サービス間の共有トークン。pingu-api が orcan-api / payment-api を呼ぶときの認証に使う(3つで同じ値)。
resource "random_password" "internal_api_token" {
  length  = 48
  special = false
}

# 秘密情報は Key Vault に置く。GitHub Actions がここから読んで、Kubernetes の Secret を作る。
# 名前は、デプロイのワークフロー(.github/workflows/production-deploy.yml)と合わせる。
module "keyvault" {
  source               = "./keyvault"
  name                 = var.key_vault_name
  location             = var.location
  resource_group_name  = azurerm_resource_group.production.name
  reader_principal_ids = var.deployer_principal_ids
  tags                 = local.tags

  secrets = {
    "internal-api-token"        = random_password.internal_api_token.result
    "orcan-database-url"        = module.postgres.database_urls["orcan"]
    "payment-database-url"      = module.postgres.database_urls["payment"]
    "pingu-database-url"        = module.postgres.database_urls["pingu"]
    "storage-connection-string" = module.storage.connection_string
  }
}

# ---- 権限 ----

# AKS のノードが、ACR からイメージを pull できる。
resource "azurerm_role_assignment" "aks_pulls_from_acr" {
  scope                = module.acr.id
  role_definition_name = "AcrPull"
  principal_id         = module.aks.kubelet_object_id
}

# GitHub Actions が、ACR にイメージを push できる。
resource "azurerm_role_assignment" "deployer_pushes_to_acr" {
  for_each             = toset(var.deployer_principal_ids)
  scope                = module.acr.id
  role_definition_name = "AcrPush"
  principal_id         = each.value
}

# GitHub Actions が、kubeconfig を取得して、クラスターにデプロイできる。
resource "azurerm_role_assignment" "deployer_gets_aks_credentials" {
  for_each             = toset(var.deployer_principal_ids)
  scope                = module.aks.id
  role_definition_name = "Azure Kubernetes Service Cluster User Role"
  principal_id         = each.value
}

resource "azurerm_role_assignment" "deployer_deploys_to_aks" {
  for_each             = toset(var.deployer_principal_ids)
  scope                = module.aks.id
  role_definition_name = "Azure Kubernetes Service RBAC Cluster Admin"
  principal_id         = each.value
}
