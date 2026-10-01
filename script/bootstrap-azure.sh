#!/usr/bin/env bash
# 本番(Azure)を Terraform と GitHub Actions で動かすための、最初の1回だけの準備。
#
#   bash script/bootstrap-azure.sh <subscription-id> <github-owner/repo>
#   例: bash script/bootstrap-azure.sh 00000000-0000-0000-0000-000000000000 asamigentoku/PinguCoin
#
# 事前に az login しておくこと。実行する人には、Subscription の「所有者(Owner)」と、
# Microsoft Entra ID でアプリを登録できる権限が必要。何度実行しても壊れない(あるものは作り直さない)。
#
# 作るもの:
#   1. Terraform の state を置く Storage(Terraform の管理の外に置く。Terraform 自身が使うため)
#   2. GitHub Actions がパスワードなしで(OIDC で)Azure にログインするための、アプリ(ID)を2つ
#        pingucoin-github-terraform ... インフラを作る(Terraform の plan / apply)。強い権限
#        pingucoin-github-deploy    ... アプリをデプロイする。権限は Terraform が最小限に絞って付ける
#
# 最後に、GitHub と backend.hcl / terraform.tfvars に設定する値を表示する。
set -euo pipefail

SUBSCRIPTION_ID="${1:-}"
GITHUB_REPO="${2:-}"
LOCATION="${LOCATION:-japaneast}"
STATE_RG="${STATE_RG:-pingucoin-tfstate-rg}"
# Storage Account の名前は世界で一意。指定がなければ、サブスクリプションIDから作る(同じ入力なら同じ名前になる)。
STATE_SA="${STATE_SA:-pingucoinstate$(printf '%s' "$SUBSCRIPTION_ID" | tr -d '-' | cut -c1-8)}"
STATE_CONTAINER="tfstate"
TERRAFORM_APP="pingucoin-github-terraform"
DEPLOY_APP="pingucoin-github-deploy"

if [ -z "$SUBSCRIPTION_ID" ] || [ -z "$GITHUB_REPO" ]; then
  echo "usage: bash script/bootstrap-azure.sh <subscription-id> <github-owner/repo>" >&2
  exit 1
fi
command -v az >/dev/null 2>&1 || { echo "az (Azure CLI) is not installed." >&2; exit 1; }
az account show >/dev/null 2>&1 || { echo "Not logged in. Run: az login" >&2; exit 1; }

az account set --subscription "$SUBSCRIPTION_ID"
TENANT_ID="$(az account show --query tenantId -o tsv)"
echo "Subscription: $SUBSCRIPTION_ID / Tenant: $TENANT_ID"

# ---- 1. Terraform の state ----
echo "== Terraform state storage ($STATE_RG / $STATE_SA)"
az group create --name "$STATE_RG" --location "$LOCATION" --output none
if ! az storage account show --name "$STATE_SA" --resource-group "$STATE_RG" >/dev/null 2>&1; then
  az storage account create \
    --name "$STATE_SA" --resource-group "$STATE_RG" --location "$LOCATION" \
    --sku Standard_GRS --kind StorageV2 --min-tls-version TLS1_2 \
    --allow-blob-public-access false --output none
fi
# 誤って上書き・削除しても戻せるようにする。
az storage account blob-service-properties update \
  --account-name "$STATE_SA" --resource-group "$STATE_RG" \
  --enable-versioning true --enable-delete-retention true --delete-retention-days 30 --output none
az storage container create --name "$STATE_CONTAINER" --account-name "$STATE_SA" --auth-mode login --output none 2>/dev/null || true
STATE_SA_ID="$(az storage account show --name "$STATE_SA" --resource-group "$STATE_RG" --query id -o tsv)"

# ---- 2. GitHub Actions 用のアプリ(OIDC) ----
ensure_app() {  # $1 = 表示名。アプリの appId を返す(なければ作る)
  local name="$1" app_id
  app_id="$(az ad app list --display-name "$name" --query "[0].appId" -o tsv)"
  if [ -z "$app_id" ]; then
    app_id="$(az ad app create --display-name "$name" --query appId -o tsv)"
  fi
  if [ -z "$(az ad sp list --filter "appId eq '$app_id'" --query "[0].id" -o tsv)" ]; then
    az ad sp create --id "$app_id" --output none
  fi
  echo "$app_id"
}

ensure_federated_credential() {  # $1 = appId, $2 = 名前, $3 = GitHub 側の subject
  local app_id="$1" name="$2" subject="$3"
  if [ -z "$(az ad app federated-credential list --id "$app_id" --query "[?name=='$name'].name" -o tsv)" ]; then
    az ad app federated-credential create --id "$app_id" --parameters "{
      \"name\": \"$name\",
      \"issuer\": \"https://token.actions.githubusercontent.com\",
      \"subject\": \"$subject\",
      \"audiences\": [\"api://AzureADTokenExchange\"]
    }" --output none
  fi
}

echo "== GitHub Actions identities"
TERRAFORM_APP_ID="$(ensure_app "$TERRAFORM_APP")"
DEPLOY_APP_ID="$(ensure_app "$DEPLOY_APP")"
TERRAFORM_SP_ID="$(az ad sp show --id "$TERRAFORM_APP_ID" --query id -o tsv)"
DEPLOY_SP_ID="$(az ad sp show --id "$DEPLOY_APP_ID" --query id -o tsv)"

# GitHub の「どのワークフローから来たか」で、ログインを許可する。リポジトリと環境(Environment)で絞る。
#   production-infra ... terraform apply(承認が要る環境)
#   pull_request / production ブランチ ... terraform plan(何が変わるかの確認)
ensure_federated_credential "$TERRAFORM_APP_ID" "github-production-infra" "repo:${GITHUB_REPO}:environment:production-infra"
ensure_federated_credential "$TERRAFORM_APP_ID" "github-pull-request" "repo:${GITHUB_REPO}:pull_request"
#   production ブランチ ... push 後の plan(承認の前に、何が変わるかを確認する)。main では、本番にログインできない
ensure_federated_credential "$TERRAFORM_APP_ID" "github-production-branch" "repo:${GITHUB_REPO}:ref:refs/heads/production"
#   production ... アプリのデプロイ(承認が要る環境)
ensure_federated_credential "$DEPLOY_APP_ID" "github-production" "repo:${GITHUB_REPO}:environment:production"

assign_role() {  # $1 = principal object id, $2 = ロール, $3 = scope
  az role assignment create --assignee-object-id "$1" --assignee-principal-type ServicePrincipal \
    --role "$2" --scope "$3" --output none 2>/dev/null || true  # すでにあれば何もしない
}

# Terraform 用: リソースを作る(Contributor)+ ロールを割り当てる(User Access Administrator)。
# ロールの割り当て(ACR・AKS・Key Vault の権限)も Terraform が行うため、後者が要る。
echo "== Role assignments"
assign_role "$TERRAFORM_SP_ID" "Contributor" "/subscriptions/${SUBSCRIPTION_ID}"
assign_role "$TERRAFORM_SP_ID" "User Access Administrator" "/subscriptions/${SUBSCRIPTION_ID}"
# state の読み書き(アクセスキーではなく、Azure の権限で行う)。
assign_role "$TERRAFORM_SP_ID" "Storage Blob Data Contributor" "$STATE_SA_ID"
# 実行している人も、ローカルで terraform が使えるようにする。
ME="$(az ad signed-in-user show --query id -o tsv 2>/dev/null || true)"
if [ -n "$ME" ]; then
  az role assignment create --assignee-object-id "$ME" --assignee-principal-type User \
    --role "Storage Blob Data Contributor" --scope "$STATE_SA_ID" --output none 2>/dev/null || true
fi
# デプロイ用の権限(ACR への push、AKS へのデプロイ、Key Vault の読み取り)は、リソースができたあとに Terraform が付ける
# (deployer_principal_ids に、下のオブジェクト ID を渡す)。ここでは何も付けない。

cat <<EOF

============================================================
完了。次の値を設定してください。
============================================================

[1] GitHub: Settings > Environments で、次の2つの環境を作る(production-infra / production は、承認者を設定すること)

  環境 production-infra (Terraform) の Variables
    AZURE_CLIENT_ID        = ${TERRAFORM_APP_ID}
    AZURE_TENANT_ID        = ${TENANT_ID}
    AZURE_SUBSCRIPTION_ID  = ${SUBSCRIPTION_ID}
    TFSTATE_RESOURCE_GROUP = ${STATE_RG}
    TFSTATE_STORAGE_ACCOUNT= ${STATE_SA}
    TFSTATE_CONTAINER      = ${STATE_CONTAINER}
    TFVARS                 = (platform/terraform/envs/production/terraform.tfvars.example を埋めた内容)

  PR の terraform plan も同じ値を使うので、上の Variables は、リポジトリの Variables(Settings > Secrets and variables > Actions)にも同じ値で設定する

  環境 production (デプロイ) の Variables
    AZURE_CLIENT_ID        = ${DEPLOY_APP_ID}
    AZURE_TENANT_ID        = ${TENANT_ID}
    AZURE_SUBSCRIPTION_ID  = ${SUBSCRIPTION_ID}
    (ほかの値は、terraform apply のあとに output から設定する。docs/DEPLOYMENT.md を参照)

[2] platform/terraform/envs/production/terraform.tfvars に、デプロイ用アプリのオブジェクト ID を書く

    deployer_principal_ids = ["${DEPLOY_SP_ID}"]

[3] ローカルで terraform を使うときの backend.hcl(platform/terraform/envs/production/backend.hcl)

    resource_group_name  = "${STATE_RG}"
    storage_account_name = "${STATE_SA}"
    container_name       = "${STATE_CONTAINER}"
    key                  = "production.tfstate"
    use_azuread_auth     = true
EOF
