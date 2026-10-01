# PinguCoin
ポイント型ECサイト、オンライン商品が実際に購入することができる

## ディレクトリ構成

[mercari-microservices-example](https://github.com/mercari/mercari-microservices-example) を参考にした、マイクロサービスのモノレポ構成です。Go のモジュールはルートに1つで、3つのサービスが `pkg/` を共有します。

```text
.
├── services/                 # マイクロサービス(それぞれが1つのAPI)
│   ├── orcan-api/            #   商品・ユーザー・在庫・アセット(gRPC)
│   │   ├── cmd/api/          #     エントリーポイント(main.go)
│   │   ├── internal/         #     このサービスだけが使うコード(grpcserver / repository / model など)
│   │   ├── proto/            #     このサービスのprotobuf(.protoと、生成したGoのコード)
│   │   ├── Dockerfile
│   │   └── .env.example
│   ├── payment-api/          #   決済・返金・ポイント(gRPC)
│   └── pingu-api/            #   ゲートウェイ(GraphQL / REST)。上の2つを呼び出す
├── pkg/                      # 複数のサービスで共有するGoのコード
│   ├── interceptor/          #   gRPCのサービス間認証・リクエストログ
│   ├── health/               #   gRPCのヘルスチェック(grpc.health.v1)
│   ├── migrate/              #   起動時のマイグレーションを、複数のPodが同時に実行しても衝突させない(DBのロック)
│   ├── gormlogger/           #   GORMのログをslog(JSON)に流すロガー
│   └── testutil/             #   テスト用のDB・偽のDB
├── apps/                     # フロントエンド(Next.js)
│   ├── client-web/           #   購入者・出品者向けのサイト
│   └── admin-web/            #   管理画面
├── platform/                 # 実行基盤(インフラ)の定義
│   ├── kubernetes/           #   Kubernetesのマニフェスト(kustomize)。環境ごとに完結していて、共有しない
│   │   ├── minikube/         #     ローカル用
│   │   └── production/       #     本番(AKS)用
│   ├── terraform/envs/       #   staging(Supabase / Azure Blob)と production(AKS / PostgreSQL / Blob / ACR / Key Vault)
│   └── docker/               #   ローカル用のPostgres / Redisなど(docker compose)
├── .github/                  # GitHub Actions
│   ├── workflows/            #   ci.yml と _go-test.yml(共通)、production-terraform.yml / production-deploy.yml(本番)
│   └── actions/production/   #   本番だけの処理の部品(AKSへの接続、Secretの作成、適用、ロールバックなど)
├── script/                   # 起動・停止・Azureの初期設定のスクリプト
├── docs/                     # 設計・API仕様・テスト・デプロイの説明
├── buf.yaml                  # protobufのワークスペース(各サービスのproto/を束ねる)
├── go.mod / go.sum           # Goのモジュール(ルートに1つ)
├── Makefile                  # 開発用コマンド(make help)
└── mise.toml                 # ツールのバージョン(go / buf / grpcurl)
```

### 依存の向き

```text
apps/*  ──GraphQL/REST──▶  pingu-api ──gRPC──▶  orcan-api
                                      └─gRPC──▶  payment-api
```

- `pingu-api` は、`orcan-api` / `payment-api` が生成した protobuf のコード(`services/*/proto`)をそのまま import します。コピーは持ちません。
- `internal/` は、そのサービスの外から import できません。複数のサービスで使うコードだけを `pkg/` に置きます。
- エラー型(`internal/apperr`)と設定(`internal/config`)は、サービスごとに性質が違う(gRPC と HTTP、環境変数の項目)ので、サービスの中に置いています。

## よく使うコマンド

```bash
make help       # コマンド一覧
make build      # 全サービスをビルド
make test       # 単体テスト
make test-db    # Postgresを起動して、統合テストを含む全テスト
make cover      # テストのカバレッジを測る(coverage.html に行ごとのレポート)
make cover-db   # Postgresも起動して、統合テストを含めたカバレッジを測る
make proto      # .protoからGoのコードを生成し直す
make up         # minikubeに起動(イメージのビルドから)
make down       # minikubeのAPIを停止
```

- 本番(Azure)へのデプロイ: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) / 月額コストの想定: [docs/COST.md](docs/COST.md)
- バージョン管理(リリース・API・DBスキーマ): [docs/VERSIONING.md](docs/VERSIONING.md)
- ログの形式(Datadogの標準属性、リクエストID): [docs/LOGGING.md](docs/LOGGING.md)
- 依存関係の自動更新(Dependabot): [docs/DEPENDABOT.md](docs/DEPENDABOT.md)
- リトライの方針(何を・どう再試行し、何を再試行しないか): [docs/RETRY.md](docs/RETRY.md)
- テストの詳細: [docs/TESTING.md](docs/TESTING.md)
- minikubeでの起動: [platform/kubernetes/minikube/README.md](platform/kubernetes/minikube/README.md)
- API仕様: [docs/API_SPEC.md](docs/API_SPEC.md) / DB: [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md)
