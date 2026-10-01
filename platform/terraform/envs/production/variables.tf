variable "azure_subscription_id" {
  description = "本番のリソースを作る Azure Subscription の ID。"
  type        = string
}

variable "location" {
  description = "リソースを作る Azure のリージョン。"
  type        = string
  default     = "Japan East"
}

variable "name_prefix" {
  description = "リソース名の接頭辞(小文字・数字・ハイフン)。"
  type        = string
  default     = "pingucoin-prod"
}

# ---- 世界で一意な名前が必要なもの ----

variable "storage_account_name" {
  description = "商品の画像・ファイルを置く Storage Account の名前(世界で一意、3〜24文字の小文字と数字)。"
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9]{3,24}$", var.storage_account_name))
    error_message = "storage_account_name は 3〜24 文字の小文字と数字だけにしてください。"
  }
}

variable "acr_name" {
  description = "コンテナイメージを置く Azure Container Registry の名前(世界で一意、5〜50文字の英数字)。"
  type        = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9]{5,50}$", var.acr_name))
    error_message = "acr_name は 5〜50 文字の英数字だけにしてください。"
  }
}

variable "key_vault_name" {
  description = "秘密情報を置く Key Vault の名前(世界で一意、3〜24文字の英数字とハイフン)。"
  type        = string
  validation {
    condition     = can(regex("^[a-zA-Z][a-zA-Z0-9-]{1,22}[a-zA-Z0-9]$", var.key_vault_name))
    error_message = "key_vault_name は 3〜24 文字で、英字で始まる英数字とハイフンにしてください。"
  }
}

variable "postgres_server_name" {
  description = "PostgreSQL サーバーの名前(世界で一意、3〜63文字の小文字・数字・ハイフン)。"
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$", var.postgres_server_name))
    error_message = "postgres_server_name は 3〜63 文字の小文字・数字・ハイフンにしてください。"
  }
}

# ---- Storage ----

variable "storage_cors_allowed_origins" {
  description = "ブラウザから Azure Blob に直接アップロードする Web アプリの Origin(末尾に / を付けない)。"
  type        = list(string)
  validation {
    condition     = length(var.storage_cors_allowed_origins) > 0 && alltrue([for origin in var.storage_cors_allowed_origins : !endswith(origin, "/")])
    error_message = "storage_cors_allowed_origins には、末尾に / を付けない Origin を1つ以上指定してください。"
  }
}

variable "storage_replication_type" {
  description = "Storage の冗長化。LRS = 同じ場所に3つのコピー(最安)、ZRS = 3つのゾーンに分散、GZRS = ZRS + 別リージョンにも複製。"
  type        = string
  default     = "LRS"
}

# ---- PostgreSQL ----

variable "postgres_sku_name" {
  description = "PostgreSQL のサイズ。B_Standard_B1ms = バースト可能(最安、1 vCPU / 2 GiB)。負荷が増えたら GP_Standard_D2ds_v5 などに上げる。"
  type        = string
  default     = "B_Standard_B1ms"
}

variable "postgres_storage_mb" {
  description = "PostgreSQL のストレージ容量(MB)。"
  type        = number
  default     = 32768
}

variable "postgres_high_availability" {
  description = "高可用性。Disabled = 1台だけ(最安。バースト可能なサイズでは、これしか選べない)、ZoneRedundant = 別ゾーンに待機系を持つ(障害時に自動で切り替わる。料金は約2倍)。"
  type        = string
  default     = "Disabled"
  validation {
    condition     = contains(["ZoneRedundant", "SameZone", "Disabled"], var.postgres_high_availability)
    error_message = "postgres_high_availability は ZoneRedundant / SameZone / Disabled のどれかにしてください。"
  }
}

variable "postgres_backup_retention_days" {
  description = "バックアップを残す日数(7〜35)。"
  type        = number
  default     = 7
}

variable "postgres_geo_redundant_backup" {
  description = "バックアップを別リージョンにも複製するか(リージョン全体の障害に備える。有効にすると料金が増える)。"
  type        = bool
  default     = false
}

# ---- AKS ----

variable "aks_kubernetes_version" {
  description = "Kubernetes のバージョン。null なら AKS の既定(推奨)バージョン。"
  type        = string
  default     = null
}

variable "aks_node_vm_size" {
  description = "ノードの VM サイズ。Standard_B2ms = バースト可能(2 vCPU / 8 GiB)。3つの API とシステムの Pod が載る最小限。"
  type        = string
  default     = "Standard_B2ms"
}

variable "aks_node_count" {
  description = "ノードの数。コストを抑えるため 1(ノードが落ちると、再作成されるまでサービスが止まる)。"
  type        = number
  default     = 1
}

variable "aks_sku_tier" {
  description = "AKS のコントロールプレーンの SLA。Free = 無料(SLA なし)、Standard = 有料(SLA 99.95%)。"
  type        = string
  default     = "Free"
  validation {
    condition     = contains(["Free", "Standard"], var.aks_sku_tier)
    error_message = "aks_sku_tier は Free か Standard にしてください。"
  }
}

variable "aks_enable_container_insights" {
  description = "AKS のログとメトリクスを Log Analytics に送る(Container Insights)。便利だが、ログの量に応じて料金がかかる。"
  type        = bool
  default     = false
}

variable "acr_sku" {
  description = "Container Registry のプラン。Basic = 最安(10 GiB まで)。"
  type        = string
  default     = "Basic"
}

variable "aks_admin_group_object_ids" {
  description = "クラスターの管理者にする Microsoft Entra ID のグループのオブジェクト ID(kubectl を使う人たち)。"
  type        = list(string)
  default     = []
}

# ---- デプロイする側(GitHub Actions) ----

variable "deployer_principal_ids" {
  description = "GitHub Actions が Azure にログインするときのアプリ(サービスプリンシパル)のオブジェクト ID。script/bootstrap-azure.sh が表示する。ACR への push、AKS へのデプロイ、Key Vault の読み取りを許可する。"
  type        = list(string)
  default     = []
}
