# 商品の画像(公開)と、販売するファイル(非公開)を置く Azure Blob Storage。
# ブラウザから直接アップロードするので、CORS を許可する(署名付き URL で書き込む)。

resource "azurerm_storage_account" "this" {
  name                = var.name
  resource_group_name = var.resource_group_name
  location            = var.location

  account_kind             = "StorageV2"
  account_tier             = "Standard"
  account_replication_type = var.replication_type
  access_tier              = "Hot"

  https_traffic_only_enabled = true
  min_tls_version            = "TLS1_2"

  # 画像用のコンテナを、Blob 単位で匿名に読めるようにするため(商品ファイル用は private のまま)。
  allow_nested_items_to_be_public = true
  # アプリが接続文字列(共有キー)で、署名付き URL を発行している。
  shared_access_key_enabled = true

  blob_properties {
    # 誤って消したり上書きしたりしても、一定期間は戻せる。
    delete_retention_policy {
      days = 14
    }
    container_delete_retention_policy {
      days = 14
    }
    versioning_enabled = true

    cors_rule {
      allowed_origins    = var.cors_allowed_origins
      allowed_methods    = ["DELETE", "GET", "HEAD", "OPTIONS", "PUT"]
      allowed_headers    = ["*"]
      exposed_headers    = ["*"]
      max_age_in_seconds = 3600
    }
  }

  tags = var.tags

  lifecycle {
    prevent_destroy = true
  }
}

# 商品画像(匿名で読める)。
resource "azurerm_storage_container" "public" {
  name                  = "pingue-public"
  storage_account_id    = azurerm_storage_account.this.id
  container_access_type = "blob"

  lifecycle {
    prevent_destroy = true
  }
}

# 販売するファイル(購入者・出品者だけが、署名付き URL でダウンロードできる)。
resource "azurerm_storage_container" "private" {
  name                  = "pingue"
  storage_account_id    = azurerm_storage_account.this.id
  container_access_type = "private"

  lifecycle {
    prevent_destroy = true
  }
}
