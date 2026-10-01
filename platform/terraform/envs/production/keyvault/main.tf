# 秘密情報(DB の接続URL、Storage の接続文字列、サービス間の共有トークン)の置き場。
# GitHub Actions が、デプロイのたびにここから読んで、Kubernetes の Secret(<api>-env)を作る。
# 権限は RBAC で管理する(アクセスポリシーは使わない)。

data "azurerm_client_config" "current" {}

resource "azurerm_key_vault" "this" {
  name                = var.name
  location            = var.location
  resource_group_name = var.resource_group_name
  tenant_id           = data.azurerm_client_config.current.tenant_id
  sku_name            = "standard"

  rbac_authorization_enabled = true

  # 誤って消しても、90 日は戻せる。purge protection で、完全に消すこともできなくなる。
  soft_delete_retention_days = 90
  purge_protection_enabled   = true

  tags = var.tags
}

# Terraform を実行している人(または CI)が、秘密情報を書き込めるようにする。
resource "azurerm_role_assignment" "writer" {
  scope                = azurerm_key_vault.this.id
  role_definition_name = "Key Vault Secrets Officer"
  principal_id         = data.azurerm_client_config.current.object_id
}

# GitHub Actions(デプロイ)が、秘密情報を読めるようにする。読むだけ。
resource "azurerm_role_assignment" "reader" {
  for_each             = toset(var.reader_principal_ids)
  scope                = azurerm_key_vault.this.id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = each.value
}

# ロールの割り当てが反映されるまで、少し時間がかかる(すぐに書くと 403 になることがある)。
resource "time_sleep" "role_propagation" {
  depends_on      = [azurerm_role_assignment.writer]
  create_duration = "60s"
}

resource "azurerm_key_vault_secret" "this" {
  # キー(名前)は秘密ではないので、sensitive を外して for_each に使う。値は sensitive のまま。
  for_each     = nonsensitive(toset(keys(var.secrets)))
  name         = each.value
  value        = var.secrets[each.value]
  key_vault_id = azurerm_key_vault.this.id

  depends_on = [time_sleep.role_propagation]
}
