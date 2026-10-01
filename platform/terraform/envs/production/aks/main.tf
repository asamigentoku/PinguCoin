# Kubernetes クラスター(AKS)。API(orcan / payment / pingu)がここで動く。

resource "azurerm_kubernetes_cluster" "this" {
  name                = "${var.name_prefix}-aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  dns_prefix          = var.name_prefix
  kubernetes_version  = var.kubernetes_version

  # Free = 無料(SLA なし)。コストを抑えるため。SLA が要るなら Standard(有料)にする。
  sku_tier = var.sku_tier

  # セキュリティ修正は自動で当てる(パッチバージョンだけ。メジャー・マイナーは手動で上げる)。
  automatic_upgrade_channel = "patch"
  node_os_upgrade_channel   = "NodeImage"

  # ローカルの管理者アカウント(証明書の kubeconfig)は使えなくする。Microsoft Entra ID の認証だけにする。
  local_account_disabled = true

  azure_active_directory_role_based_access_control {
    azure_rbac_enabled     = true
    admin_group_object_ids = var.admin_group_object_ids
  }

  default_node_pool {
    name           = "system"
    vm_size        = var.node_vm_size
    vnet_subnet_id = var.subnet_id

    # コストを抑えるため、ノードは固定で var.node_count 個(既定は1つ)。自動スケールもゾーン分散もしない。
    # ノードやゾーンが落ちると、再作成されるまでサービスが止まる。止めたくなくなったら、node_count を増やし、
    # zones と自動スケール(auto_scaling_enabled / min_count / max_count)を足す。
    node_count = var.node_count

    os_disk_type = "Managed"

    upgrade_settings {
      max_surge = "33%"
    }
  }

  identity {
    type = "SystemAssigned"
  }

  # ノードは、上の default_node_pool で自分たちで管理する(Manual)。自動プロビジョニング(Auto)は使わない。
  node_provisioning_profile {
    mode = "Manual"
  }

  network_profile {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    # NetworkPolicy(platform/kubernetes/production/network-policy.yaml)を効かせるために必要。
    network_data_plane = "cilium"
    network_policy     = "cilium"
    load_balancer_sku  = "standard"
    service_cidr       = "10.21.0.0/16"
    dns_service_ip     = "10.21.0.10"
  }

  # Pod 内のアプリが、Azure の権限を、パスワードなしで使えるようにする(今は使っていないが、将来のために有効にしておく)。
  oidc_issuer_enabled       = true
  workload_identity_enabled = true

  # ログとメトリクスを Log Analytics に送る(Container Insights)。ログの量に応じて料金がかかるので、既定では無効。
  dynamic "oms_agent" {
    for_each = var.enable_container_insights ? [1] : []
    content {
      log_analytics_workspace_id = var.log_analytics_workspace_id
    }
  }

  # Ingress(インターネットからの入口)用の、マネージドの NGINX。
  web_app_routing {
    dns_zone_ids = []
  }

  # 自動アップグレードを、アクセスの少ない時間(日本時間の月曜 午前3時台)に限る。
  maintenance_window_auto_upgrade {
    frequency   = "Weekly"
    interval    = 1
    duration    = 4
    day_of_week = "Sunday"
    start_time  = "18:00"
    utc_offset  = "+00:00"
  }

  tags = var.tags
}
