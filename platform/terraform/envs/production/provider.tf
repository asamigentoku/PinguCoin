terraform {
  required_version = ">= 1.7.0"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.12"
    }
  }

  # State は Azure Storage に置く(チームと GitHub Actions で共有し、同時実行をロックで防ぐ)。
  # 接続先は backend.hcl で渡す: terraform init -backend-config=backend.hcl
  # 作り方は script/bootstrap-azure.sh を参照。
  backend "azurerm" {}
}

# ローカルでは az login、GitHub Actions では OIDC(ARM_USE_OIDC / ARM_CLIENT_ID / ARM_TENANT_ID)で認証する。
provider "azurerm" {
  subscription_id = var.azure_subscription_id

  features {
    key_vault {
      # 削除したら、すぐ作り直せるように purge する(本番では purge protection を有効にしているため、実際には消えない)。
      purge_soft_delete_on_destroy = false
    }
    resource_group {
      # 中にリソースが残っているリソースグループを、うっかり消さない。
      prevent_deletion_if_contains_resources = true
    }
  }
}
