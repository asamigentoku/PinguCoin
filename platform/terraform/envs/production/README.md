# production infrastructure

PinguCoin の本番環境(AKS / Azure Database for PostgreSQL / Azure Blob Storage / Container Registry / Key Vault)を、Terraform で管理します。

手順の全体(初回のセットアップ、GitHub の設定、デプロイ、運用)は [docs/DEPLOYMENT.md](../../../../docs/DEPLOYMENT.md) を参照してください。

## 構成

```text
production/
├── main.tf        # 子モジュールの呼び出しと、権限(ACR・AKS)
├── provider.tf    # Provider、state の保存先(Azure Storage)
├── variables.tf   # 入力値(既定値は、コストを最小にした構成)
├── outputs.tf     # デプロイ(GitHub Actions)が使う値
├── network/       # 仮想ネットワーク、DB 用のプライベート DNS
├── aks/           # AKS(ノード1つ)
├── postgres/      # PostgreSQL Flexible Server と3つのデータベース
├── storage/       # Blob Storage(商品の画像・ファイル)
├── acr/           # Container Registry
├── keyvault/      # Key Vault(接続URLなどの秘密情報)
└── monitoring/    # Log Analytics
```

- State は Azure Storage に置きます。接続先は `backend.hcl` で渡します(`backend.hcl.example` をコピー)。
- 通常は、GitHub Actions(`.github/workflows/production-terraform.yml`)が実行します。ローカルで試すときは次のとおりです。

```bash
az login
cp backend.hcl.example backend.hcl          # script/bootstrap-azure.sh が表示した値に置き換える
cp terraform.tfvars.example terraform.tfvars # 実際の値に置き換える
terraform init -backend-config=backend.hcl
terraform plan
```

`backend.hcl` と `terraform.tfvars` は、Git の管理対象外です。
