# staging infrastructure

PinguCoinのstaging環境で使用するSupabaseとAzure Blob StorageをTerraformで管理します。

## ディレクトリ構成

```text
staging/
├── main.tf                 # 子モジュールの呼び出し
├── provider.tf             # Providerとバージョン
├── variables.tf            # staging共通の入力値
├── outputs.tf              # デプロイ後の出力値
├── supabase/               # Supabase Project
└── azure-blob/             # Resource Group、Storage Account、Container
```

ルートの`main.tf`には各サービスの呼び出しだけを置き、リソース定義はサービス別ディレクトリに分離しています。

既存のSupabase Projectは`moved`ブロックで新しいモジュールアドレスへ移行します。手動の`terraform state mv`やProjectの再作成は不要です。

## 事前準備

- Terraform 1.7以上
- Azure CLI
- AzureとSupabaseにリソースを作成できる権限
- Supabase Personal Access Token

Azureへログインして対象Subscriptionを選択します。

```powershell
az login
az account set --subscription "<subscription-id>"
```

Supabase DashboardのAccount preferences > Access Tokensでトークンを作成し、環境変数へ設定します。

```powershell
$env:SUPABASE_ACCESS_TOKEN = "<supabase-access-token>"
```

## 設定

サンプルをコピーし、実際の値へ置き換えます。`terraform.tfvars`はGit管理対象外です。

```powershell
Copy-Item terraform.tfvars.example terraform.tfvars
```

主な入力値は次のとおりです。

| 変数 | 内容 |
| --- | --- |
| `organization_id` | Supabase Organization ID |
| `database_password` | Supabaseの`postgres`ユーザー用パスワード |
| `azure_subscription_id` | Azure Subscription ID |
| `azure_storage_account_name` | 世界で一意な3～24文字の小文字・数字の名前 |
| `storage_cors_allowed_origins` | Blobへ直接アクセスするWebアプリのOrigin |

Originの末尾に`/`は付けません。Vercelのstagingドメインが確定したら、サンプル値を実際のURLへ置き換えてください。

## 実行

```powershell
terraform init
terraform plan
terraform apply
```

出力値を確認できます。

```powershell
terraform output
terraform output -raw azure_storage_connection_string
```

Azure Blobを利用するAPIには次の環境変数を設定します。

```text
AZURE_STORAGE_CONNECTION_STRING=<terraform outputの値>
AZURE_STORAGE_PUBLIC_CONTAINER=pingue-public
AZURE_STORAGE_PRIVATE_CONTAINER=pingue
APP_ENV=staging
```

3つのAPIには、SupabaseのConnect画面で取得した同一のTransaction pooler URLを`DATABASE_URL`として設定します。DB名は`postgres`です。

## セキュリティ上の注意

- `azure_storage_connection_string`とTerraform stateには機密情報が含まれます。コミットしないでください。
- チーム運用では、暗号化とアクセス制御を行ったRemote Stateへの移行を推奨します。
- 現在のアプリ実装に合わせ、公開商品画像とShared KeyによるSAS発行を有効にしています。
- `terraform destroy`はstagingのSupabase DBとAzure Storage内のBlobを含むリソースを削除します。
- Supabase DBパスワードの変更はSupabase Dashboardから行ってください。
