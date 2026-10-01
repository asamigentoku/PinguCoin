output "fqdn" {
  description = "仮想ネットワークの中だけで引ける、DB のホスト名。"
  value       = azurerm_postgresql_flexible_server.this.fqdn
}

output "admin_login" {
  value = local.admin
}

output "admin_password" {
  value     = random_password.admin.result
  sensitive = true
}

output "database_names" {
  value = local.databases
}

# サービスごとの接続URL(Key Vault に入れて、アプリの DATABASE_URL になる)。
output "database_urls" {
  value = {
    for name in local.databases :
    name => "postgres://${local.admin}:${random_password.admin.result}@${azurerm_postgresql_flexible_server.this.fqdn}:5432/${name}?sslmode=require"
  }
  sensitive = true
}
