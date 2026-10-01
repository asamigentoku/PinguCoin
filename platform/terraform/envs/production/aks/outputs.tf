output "id" {
  value = azurerm_kubernetes_cluster.this.id
}

output "name" {
  value = azurerm_kubernetes_cluster.this.name
}

output "kubelet_object_id" {
  description = "ノードが ACR からイメージを pull するときに使う ID(AcrPull を割り当てる)。"
  value       = azurerm_kubernetes_cluster.this.kubelet_identity[0].object_id
}
