# 本番へのデプロイ(Azure)

API(`orcan-api` / `payment-api` / `pingu-api`)を、Azure の AKS で動かします。インフラは Terraform、デプロイは GitHub Actions です。

> **この構成は、コストを最小にするため、すべて「1つだけ」です。** Pod もノードも DB も冗長化していないので、障害のときはサービスが止まります(下の「止まるとき」)。利用者が増えたら、[止まりにくくする](#止まりにくくする)で上げてください。

## 全体像

```text
GitHub Actions ──(OIDC。パスワードなし)──▶ Azure     ※ production ブランチへの push だけが、本番に反映される
  ├─ production-terraform.yml : インフラを作る・変える(承認が要る)
  └─ production-deploy.yml    : イメージを ACR に push → AKS にデプロイ(承認が要る)

インターネット ─HTTPS─▶ Ingress(マネージドNGINX)─▶ pingu-api ─gRPC─▶ orcan-api  ─▶ Azure Blob
                                                         └─gRPC─▶ payment-api
                       3つのAPI ──(仮想ネットワークの中だけ)──▶ Azure Database for PostgreSQL
```

| 部品 | Azure のリソース | 定義 |
| --- | --- | --- |
| Kubernetes | AKS(ノード1つ) | `platform/terraform/envs/production/aks` |
| データベース | Azure Database for PostgreSQL Flexible Server(`orcan` / `payment` / `pingu` の3つ) | `.../postgres` |
| 商品の画像・ファイル | Azure Blob Storage(`pingue-public` / `pingue`) | `.../storage` |
| イメージの置き場 | Azure Container Registry | `.../acr` |
| 秘密情報 | Azure Key Vault | `.../keyvault` |
| ネットワーク | 仮想ネットワーク、DB 用のプライベート DNS | `.../network` |
| アプリの定義 | Kubernetes のマニフェスト(kustomize。minikube とは共有せず、このフォルダだけで完結) | `platform/kubernetes/production` |

- DB は**インターネットに公開しません**(仮想ネットワークの中だけ)。AKS からだけつながります。
- 秘密情報(DB の接続URL、Storage の接続文字列、サービス間のトークン、Grafana の管理者パスワード)は Terraform が作って Key Vault に入れ、デプロイのたびに GitHub Actions が読んで Kubernetes の Secret にします。

## コスト最小の構成

| | 既定 | 上げるときの変数(`terraform.tfvars`) |
| --- | --- | --- |
| API の Pod | 各サービス 1 つ(HPA・PDB なし) | `platform/kubernetes/production/servers.yaml` の replicas と、同じフォルダに HPA・PDB を足す |
| AKS のノード | 1 つ、`Standard_B2ms`、SLA なし(Free) | `aks_node_count`、`aks_sku_tier = "Standard"` |
| PostgreSQL | `B_Standard_B1ms`、高可用性なし、バックアップ 7 日(同じリージョン内) | `postgres_sku_name`、`postgres_high_availability`、`postgres_geo_redundant_backup` |
| Storage | LRS(同じ場所に3コピー) | `storage_replication_type = "ZRS"` / `"GZRS"` |
| Container Registry | Basic | `acr_sku` |
| ログ | Container Insights は無効(ログの量に応じた料金がかからない) | `aks_enable_container_insights = true` |

月額の想定(約 145 ドル)と内訳は [COST.md](COST.md) を参照してください。主な費用は、AKS のノード、PostgreSQL、Ingress 用の公開 IP とロードバランサーです。

### 止まるとき

1つしかないので、次のときは、リクエストが失敗します。

- **デプロイ**: 止まりません。新しい Pod が Ready になってから、古い Pod を止めます(`maxUnavailable: 0`)。
- **Pod が落ちた**: 自動で再作成されるまでの、数十秒。
- **ノードが落ちた・AKS のアップグレードでノードを入れ替えた**: 新しいノードで Pod が立ち上がるまでの、数分。自動アップグレードは、日本時間の月曜 午前3時台に限っています。
- **DB の障害・メンテナンス**: 復旧するまで(高可用性がないため)。Azure のメンテナンスも、数十秒〜数分止まることがあります。
- **ゾーンやリージョン全体の障害**: 復旧するまで。

### 止まりにくくする

**DB まわりは、方針として最安のままにします**(上の表の PostgreSQL・バックアップの行。`B_Standard_B1ms`、ストレージは最小の 32GB、高可用性なし、バックアップ 7 日・同じリージョン内)。これより安い選択肢は、Azure にありません。
DB のデータが失われたときは、バックアップ(最大 7 日前まで、5 分単位で戻せる)から復元します。リージョン全体の障害には備えていません。

DB 以外は、必要なところから、順番に上げられます。目安は次のとおりです。

1. **AKS**: `aks_node_count = 3`(あわせて、`aks/main.tf` に `zones = ["1","2","3"]` を足す)と `aks_sku_tier = "Standard"`。
2. **Pod**: `platform/kubernetes/production/servers.yaml` の `replicas` を 2 にして、HPA と PDB(`autoscaling.yaml`)を同じフォルダに作り、`kustomization.yaml` の `resources` に足す(minikube の `platform/kubernetes/minikube/autoscaling.yaml` を参考にできる)。

## ワークフローの構成

GitHub Actions は、`.github/workflows/` の**直下**にあるファイルしか読みません(サブフォルダには置けません)。そこで、次のように分けています。

```text
.github/
├── workflows/
│   ├── ci.yml                    # 共通: PR と main / production への push で、テスト・検証をする
│   ├── _go-test.yml              # 共通: Go のビルド・テスト(ci.yml と production-deploy.yml が呼び出す)
│   ├── production-terraform.yml  # 本番: インフラ(plan / apply)
│   └── production-deploy.yml     # 本番: アプリのデプロイ
└── actions/production/           # 本番だけの処理の部品(composite action)
    ├── terraform-init/           #   state に接続して init、tfvars を作る
    ├── connect-aks/              #   AKS に接続する(kubectl / helm / kustomize の準備を含む)
    ├── install-cert-manager/     #   cert-manager と ClusterIssuer
    ├── create-secrets/           #   Key Vault の値から Kubernetes の Secret を作る
    ├── apply-manifests/          #   イメージのタグを差し替えて適用し、ロールアウトを待つ
    ├── smoke-test/               #   /readyz の確認
    └── rollback/                 #   1つ前のリビジョンに戻す
```

- 本番用のワークフローは、名前の頭に `production-` を付けて、一覧でまとまるようにしています。
- ワークフローの本体(入口、承認、ブランチの制限)は直下のファイルに、本番だけの具体的な処理は `actions/production/` の部品に書いています。
- 共通の処理(Go のテスト)は、CI と本番デプロイが同じ `_go-test.yml` を使うので、本番に出す前にも、PR と同じ確認が走ります。

## ブランチの運用

| ブランチ | 動くもの |
| --- | --- |
| プルリクエスト | `ci.yml`(テスト・検証)、`production-terraform.yml` の plan(インフラを変える PR だけ。変更は加えない) |
| `main` | `ci.yml` だけ。**本番には何も反映されない** |
| `production` | `ci.yml` に加えて、`production-terraform.yml`(plan → 承認 → apply)と `production-deploy.yml`(テスト → ビルド → 承認 → デプロイ) |

- 本番に出したいときは、`main` から `production` へ、プルリクエストでマージします(`main` の変更を、`production` に取り込む)。`production` への push が、本番への反映の合図です。
- `production` ブランチは、**ブランチ保護**(直接 push の禁止、プルリクエストとレビューを必須)にしてください。
- 手動実行(Actions の Run workflow)で、`production` 以外のブランチを選んでも、apply とデプロイには進みません(ワークフローが止めます)。
- さらに、GitHub の Environment(`production-infra` / `production`)の **Deployment branches** を `production` だけに制限してください。ワークフローの書き換えでは、すり抜けられません。
- Azure 側も同じです。Terraform 用の ID が本番にログインできるのは、`production` ブランチ(plan)と、承認された環境(apply)、プルリクエスト(plan)だけです(`bootstrap-azure.sh` が設定します)。

## 初回のセットアップ

### 1. Azure の準備(1回だけ)

```bash
az login
bash script/bootstrap-azure.sh <subscription-id> <github-owner>/<repo>
```

Terraform の state を置く Storage と、GitHub Actions がログインするための ID を2つ作ります(`pingucoin-github-terraform` = インフラ用、`pingucoin-github-deploy` = デプロイ用)。最後に、GitHub などに設定する値が表示されます。

### 2. GitHub の設定

**Settings > Environments** で、次の2つを作り、**Required reviewers**(承認者)と、**Deployment branches = `production` だけ**を設定します。

| 環境 | 使うワークフロー |
| --- | --- |
| `production-infra` | `production-terraform.yml` の apply |
| `production` | `production-deploy.yml` の deploy |

**Variables**(秘密ではない値)に、スクリプトが表示した値を設定します。

| 場所 | 変数 | 値 |
| --- | --- | --- |
| リポジトリ と `production-infra` | `AZURE_CLIENT_ID` | `pingucoin-github-terraform` の ID |
| | `AZURE_TENANT_ID` / `AZURE_SUBSCRIPTION_ID` | 表示された値 |
| | `TFSTATE_RESOURCE_GROUP` / `TFSTATE_STORAGE_ACCOUNT` / `TFSTATE_CONTAINER` | 表示された値 |
| | `TFVARS` | `terraform.tfvars` の中身(次の手順) |
| `production` | `AZURE_CLIENT_ID` | `pingucoin-github-deploy` の ID |
| | `AZURE_TENANT_ID` / `AZURE_SUBSCRIPTION_ID` | 同じ値 |

> `AZURE_CLIENT_ID` は、リポジトリ(PR の plan 用)と `production-infra` に**インフラ用**の ID、`production` に**デプロイ用**の ID を設定します。

### 3. Terraform の入力値(`TFVARS`)

`platform/terraform/envs/production/terraform.tfvars.example` をもとに、実際の値を埋めます。

- `azure_subscription_id`、世界で一意な名前(`storage_account_name`、`acr_name`、`key_vault_name`、`postgres_server_name`)
- `storage_cors_allowed_origins`: Web アプリの URL(本番のドメイン)。末尾に `/` を付けない
- `deployer_principal_ids`: スクリプトが表示した、デプロイ用 ID のオブジェクト ID
- `aks_admin_group_object_ids`: `kubectl` を使う人たちの、Microsoft Entra ID のグループ

この中身を、GitHub の Variable `TFVARS` にそのまま貼り付けます(秘密は含まれません)。

### 4. インフラを作る

`production` ブランチに push するか、そのブランチで Actions の **Terraform (production)** を手動で実行します。plan の内容を確認して、`production-infra` を承認すると、`terraform apply` が走ります(10〜30分かかります)。終わると、ジョブの要約に `terraform output` が出ます。

### 5. デプロイ用の値を設定する

`terraform output` の値を、GitHub の Environment `production` の Variables に設定します。

| 変数 | 値 |
| --- | --- |
| `AZURE_RESOURCE_GROUP` | `azure_resource_group_name` |
| `AKS_CLUSTER_NAME` | `aks_cluster_name` |
| `ACR_NAME` | `acr_name` |
| `KEY_VAULT_NAME` | `key_vault_name` |
| `AZURE_STORAGE_PUBLIC_CONTAINER` / `AZURE_STORAGE_PRIVATE_CONTAINER` | 同名の output(`pingue-public` / `pingue`) |
| `PINGU_API_HOST` | API を公開するホスト名(例: `api.pingucoin.example`) |
| `LETSENCRYPT_EMAIL` | 証明書の期限切れなどの通知先 |

**Secrets**(秘密)は、`production` に1つだけ設定します。

| Secret | 値 |
| --- | --- |
| `CLERK_SECRET_KEY` | Clerk の本番のシークレットキー |

あわせて、`platform/kubernetes/production/kustomization.yaml` の `newName`(`pingucoinprodacr.azurecr.io/…`)を、実際の ACR のログインサーバー(`acr_login_server`)に書き換えます。

### 6. デプロイして、DNS を向ける

1. Actions の **Deploy (production)** を実行(`production` ブランチへの push でも動きます)。`production` を承認します。
2. 終わったら、Ingress の IP を調べます。

   ```bash
   az aks get-credentials -g <resource-group> -n <aks> && kubelogin convert-kubeconfig -l azurecli
   kubectl get ingress -n pingucoin
   ```

3. `PINGU_API_HOST` の DNS に、その IP の **A レコード**を作ります。DNS が反映されると、cert-manager が TLS 証明書を自動で取得します(数分)。
4. Web アプリ(`apps/client-web`)の環境変数 `PINGU_API_URL` を `https://<PINGU_API_HOST>/api/v1/graphql` にします。

> Web アプリ(`apps/client-web` / `admin-web`)のデプロイは、このワークフローには含まれていません(Vercel など、別の場所で動かす想定です)。

## 日常の運用

| やりたいこと | 方法 |
| --- | --- |
| API を更新する | `main` で開発 → `production` ブランチへマージ(`services/**` や `pkg/**` が変わったときに自動で動く)→ `production` を承認 |
| 前の状態に戻す | デプロイが失敗したときは自動で戻る。あとから戻すなら、前の成功したコミットで **Deploy (production)** を再実行(イメージは git の SHA でタグ付けされているので、そのまま使える) |
| Secret(Key Vault の値や `CLERK_SECRET_KEY`)を変えた | **Deploy (production)** を、`restart` にチェックを入れて手動で実行 |
| インフラを変える | `platform/terraform/envs/production/` を変えて PR を出す。PR で plan を確認し、`production` ブランチへのマージ後に `production-infra` を承認 |
| ログを見る | `kubectl logs -n pingucoin deployment/pingu-api`(Container Insights を有効にすれば、Azure のポータルでも見られる) |
| メトリクス・グラフ・アラートを見る | `bash script/monitor.sh`(Grafana: http://localhost:3001、Prometheus: http://localhost:9090)。AKS の管理者だけ。インターネットには公開していない。[MONITORING.md](MONITORING.md) |

## 注意

- **DB のユーザー**: 3つのサービスとも、同じ管理者ユーザーで接続しています。サービスごとに権限を絞ったユーザーを作るのが望ましいです(DB が仮想ネットワークの中にあり、CI から直接つながらないため、今は未対応)。
- **Terraform の state**: パスワードなどの秘密を含みます。state の Storage には、アクセスできる人を最小限にしてください(`bootstrap-azure.sh` は、アクセスキーを使わず、Azure の権限だけで読み書きするようにしています)。
- **PR の plan**: PR の plan も、インフラ用の ID(強い権限)で動きます。ワークフローを書き換える PR で悪用されないよう、`main` と `production` のブランチ保護(レビュー必須)を設定してください。
- **削除の保護**: DB、Storage、Key Vault などは `prevent_destroy` と、削除後も一定期間戻せる設定が入っています。`terraform destroy` では消えません。
- **起動時のマイグレーション**: 各サービスが、起動時に、まだ適用していない SQL のマイグレーション(`services/*/migrations/`)を、番号順に適用します。デプロイ中に古い Pod と新しい Pod が同時に動いても、DB のロックで1つずつ実行します。適用した内容は、サービスごとの記録のテーブル(`orcan_schema_migrations` など)に残ります。変更の足し方、古いアプリが動くための手順は [VERSIONING.md](VERSIONING.md) を参照。
