# Azure Database for PostgreSQL(Flexible Server)。
# 仮想ネットワークの中だけからつながる(インターネットには公開しない)。
# 3つのサービス(orcan / payment / pingu)が、それぞれ別のデータベースを使う。テーブルは各サービスが起動時に作る。

locals {
  databases = ["orcan", "payment", "pingu"]
  admin     = "pinguadmin"
}

# 記号を含めない(接続URLに入れるときに、エスケープが要らないようにする)。
resource "random_password" "admin" {
  length  = 32
  special = false
}

resource "azurerm_postgresql_flexible_server" "this" {
  name                = var.name
  location            = var.location
  resource_group_name = var.resource_group_name
  version             = "16"

  # 仮想ネットワークの中に置く(public_network_access_enabled を false にするのに必要)。
  delegated_subnet_id           = var.delegated_subnet_id
  private_dns_zone_id           = var.private_dns_zone_id
  public_network_access_enabled = false

  administrator_login    = local.admin
  administrator_password = random_password.admin.result

  sku_name   = var.sku_name
  storage_mb = var.storage_mb

  zone = "1"
  dynamic "high_availability" {
    for_each = var.high_availability == "Disabled" ? [] : [1]
    content {
      mode                      = var.high_availability
      standby_availability_zone = var.high_availability == "ZoneRedundant" ? "2" : "1"
    }
  }

  backup_retention_days        = var.backup_retention_days
  geo_redundant_backup_enabled = var.geo_redundant_backup

  tags = var.tags

  lifecycle {
    # 本番のデータごと消してしまう操作を、terraform destroy でうっかりできないようにする。
    prevent_destroy = true
    # 高可用性の切り替え(フェイルオーバー)でゾーンが入れ替わっても、差分にしない。
    ignore_changes = [zone, high_availability[0].standby_availability_zone]
  }

  # DNS ゾーンが、仮想ネットワークにつながってから作る。
  depends_on = [var.private_dns_zone_link_id]
}

resource "azurerm_postgresql_flexible_server_database" "this" {
  for_each  = toset(local.databases)
  name      = each.value
  server_id = azurerm_postgresql_flexible_server.this.id
  charset   = "UTF8"
  collation = "en_US.utf8"

  lifecycle {
    prevent_destroy = true
  }
}
