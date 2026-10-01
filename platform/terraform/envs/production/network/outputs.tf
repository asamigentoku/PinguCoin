output "aks_subnet_id" {
  value = azurerm_subnet.aks.id
}

output "postgres_subnet_id" {
  value = azurerm_subnet.postgres.id
}

output "postgres_private_dns_zone_id" {
  value = azurerm_private_dns_zone.postgres.id
}

output "postgres_private_dns_zone_link_id" {
  description = "PostgreSQL が、DNS ゾーンのリンクより後に作られるようにするための依存用。"
  value       = azurerm_private_dns_zone_virtual_network_link.postgres.id
}
