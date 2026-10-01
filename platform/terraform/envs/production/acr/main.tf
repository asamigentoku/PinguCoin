# コンテナイメージの置き場(Azure Container Registry)。
# GitHub Actions がここにイメージを push し、AKS がここから pull する。
# 認証はパスワードではなく、Azure の権限(ロール)で行う(admin ユーザーは無効)。

resource "azurerm_container_registry" "this" {
  name                = var.name
  location            = var.location
  resource_group_name = var.resource_group_name
  sku                 = var.sku
  admin_enabled       = false
  tags                = var.tags
}
