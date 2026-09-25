# terraform/envs/staging

staging 用の Supabase プロジェクト(`pingu-staging`)を Terraform で管理する。

## 準備

1. Supabase ダッシュボードの Account preferences > Access Tokens でアクセストークンを発行する。
2. `terraform.tfvars` を作成し、値を埋める(`.gitignore` 対象なのでコミットされない)。

   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

   | 変数 | 内容 |
   | --- | --- |
   | `organization_id` | Organization settings > General の Organization slug |
   | `database_password` | DB の `postgres` ユーザーのパスワード |
   | `region` | 省略時は `ap-northeast-1`(東京) |

## 実行

このディレクトリで実行する。

```bash
export SUPABASE_ACCESS_TOKEN=<アクセストークン>

terraform init    # 初回のみ
terraform plan    # 変更内容の確認
terraform apply   # 反映
terraform output  # project_ref / db_host の確認
```

## 注意

- `terraform destroy` を実行すると staging の DB ごと削除される。
- DB パスワードは作成後に `terraform.tfvars` を変更しても反映されない(`ignore_changes`)。
  変更はダッシュボードから行う。
- Supabase のプロジェクト 1 つにつき DB は `postgres` の 1 つだけ。
  pingu-api / orcan-api / payment-api で DB を分ける場合は、スキーマで分けるかプロジェクトを分ける。
