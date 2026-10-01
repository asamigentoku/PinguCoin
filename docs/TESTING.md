# バックエンドのテスト

Go のモジュールはリポジトリのルートに1つです。テストはルートから実行します。

```bash
go test ./...                      # 全部
go test ./services/orcan-api/...   # 1つのサービスだけ
go test ./pkg/...                  # 共通コード(pkg/)だけ
```

## 2種類のテスト

| 種類 | 必要なもの | 内容 |
| --- | --- | --- |
| 単体テスト | なし | 入力チェック、エラーの変換、認証、ヘルスチェック、設定の読み込み、注文・ポイントの API の振る舞い(orcan-api / payment-api は偽物に差し替え)など |
| 統合テスト | Postgres | DB 自身の挙動(行ロック、制約、トランザクションの巻き戻し、初期データ)。**`TEST_DATABASE_URL` が未設定ならスキップ**されます |

## 統合テストを動かす

Postgres を起動して、接続 URL を環境変数で渡します。

```bash
docker run -d --name pg-test -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=pingucoin_test -p 55432:5432 postgres:16-alpine

export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55432/pingucoin_test?sslmode=disable"
go test ./...
```

- テストごとに専用のスキーマ(`t_xxxxxx`)を作ってマイグレーションし、終わったら削除します。テスト同士のデータは混ざらず、同じ DB を何度使っても汚れません。
- 接続先の DB には、**staging など本物のデータが入った DB を指定しないでください**。専用のスキーマしか触りませんが、念のため、使い捨ての Postgres を使ってください。
- 終わったら `docker rm -f pg-test` で片付けます。

## カバレッジ

テストが、コードのどれだけを実行したか(カバレッジ)を測ります。

```bash
make cover            # DBなし。統合テストはスキップされる(その分は、カバーされていない扱い)
make cover-db         # Postgresを起動して、統合テストも含める
bash script/coverage.sh                    # make を使わずに、同じことをする
MIN_COVERAGE=48 bash script/coverage.sh    # 全体の割合が 48% 未満なら失敗する
```

- パッケージごとの割合と、全体の割合が表示されます。`coverage.html` を開くと、行ごとに、通った所(緑)と通っていない所(赤)が見られます(`coverage.out` / `coverage.html` は Git の管理対象外)。
- 数え方: サービスと `pkg/` の全体を対象にして、あるパッケージのテストが別のパッケージのコードを通ったら、それも数えます。自動生成のコード(`*.pb.go`、`graph/generated.go` など)、`cmd/`(main)、`pkg/testutil` は、除いています。
- **CI** では、`_go-test.yml` が `script/coverage.sh` を実行します(Postgres つき)。全体の割合が下限(`MIN_COVERAGE`、いま 48%)を下回ると失敗し、`coverage.html` は、実行結果の **Artifacts** からダウンロードできます。割合は、ジョブの要約にも出ます。
- 下限は、カバレッジが知らないうちに下がらないための番人です。テストを増やしたら、少しずつ上げてください。

### 現在の数字(DBありで、全体 約 53%)

| 割合 | パッケージ |
| --- | --- |
| 90〜100% | `pkg/*`(interceptor, health, gormlogger)、`*/config`、`*/apperr`、`payment-api/repository`、`pingu-api/{grpcclient,orcanclient,paymentclient,reqcontext}` |
| 50〜80% | `payment-api/grpcserver`、`orcan-api/database`、`pkg/migrate`、`pingu-api/httpapi` |
| 低い(優先して増やしたい所) | `orcan-api/grpcserver`(約 10%。商品・カテゴリー・ユーザーなどの CRUD)、`orcan-api/repository`(約 30%)、`pingu-api/graph/resolvers`(約 31%)、`pingu-api/clerkauth`(約 14%) |

## 何をテストしているか

- **orcan-api**: 在庫の消費(冪等性、在庫不足、同時購入でも売りすぎない)、商品を作ると在庫が作られること、起動時のカテゴリー初期データと既存商品への在庫の補完、gRPC のサービス間認証とログ、ヘルスチェック(DB が落ちたときに readiness だけが落ちる)。
- **payment-api**: ポイント払い(残高の消費、足りないときは決済ごと巻き戻る、同じキーの再送で二重課金しない、返金で残高が戻る)、手動の付与・消費、同時消費でも残高がマイナスにならない。
- **pingu-api**: 注文の流れ(在庫 → 決済 → 注文の記録の順、冪等性キー、在庫不足なら決済に進まない、決済失敗で在庫を戻す)、他人の注文は見られない、ポイント残高とウェルカムボーナス(1回だけ)、商品の更新・削除は出品者だけ、非公開ファイルの閲覧・ダウンロードは出品者と購入者だけ、エラーの HTTP への変換と内部情報を隠すこと。

## 補助部品

`pkg/testutil` は、3つのサービスで共通に、テストでだけ使います。

- `NewDB(t, database.Migrate)`: 上の専用スキーマを作って、**本番と同じ SQL のマイグレーション**を適用し、接続した `*gorm.DB` を返します。
- `NewFakeDB(t)`: Postgres なしで作れる、`Ping` の成否だけを切り替えられる偽の DB です(ヘルスチェックのテスト用)。

## Windows でテスト用バイナリが実行できないとき

環境によっては、「アプリケーション制御ポリシーによってこのファイルがブロックされました」と出て、`go test` が失敗することがあります。テストの失敗ではなく、一時フォルダに作られたテスト用バイナリが OS に止められています。別の場所にビルドして実行すると通ることがあります。

```bash
go test -c -o ./out.test.exe ./services/orcan-api/internal/repository
cd services/orcan-api/internal/repository && ../../../../out.test.exe -test.v
```
