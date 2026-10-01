# バックエンドのテスト

3つのサービス(`orcan-api` / `payment-api` / `pingu-api`)は、それぞれ別の Go モジュールです。テストはサービスのフォルダで実行します。

```bash
cd server/orcan-api && go test ./...
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
cd server/payment-api && go test ./...
```

- テストごとに専用のスキーマ(`t_xxxxxx`)を作ってマイグレーションし、終わったら削除します。テスト同士のデータは混ざらず、同じ DB を何度使っても汚れません。
- 接続先の DB には、**staging など本物のデータが入った DB を指定しないでください**。専用のスキーマしか触りませんが、念のため、使い捨ての Postgres を使ってください。
- 終わったら `docker rm -f pg-test` で片付けます。

## 何をテストしているか

- **orcan-api**: 在庫の消費(冪等性、在庫不足、同時購入でも売りすぎない)、商品を作ると在庫が作られること、起動時のカテゴリー初期データと既存商品への在庫の補完、gRPC のサービス間認証とログ、ヘルスチェック(DB が落ちたときに readiness だけが落ちる)。
- **payment-api**: ポイント払い(残高の消費、足りないときは決済ごと巻き戻る、同じキーの再送で二重課金しない、返金で残高が戻る)、手動の付与・消費、同時消費でも残高がマイナスにならない。
- **pingu-api**: 注文の流れ(在庫 → 決済 → 注文の記録の順、冪等性キー、在庫不足なら決済に進まない、決済失敗で在庫を戻す)、他人の注文は見られない、ポイント残高とウェルカムボーナス(1回だけ)、商品の更新・削除は出品者だけ、非公開ファイルの閲覧・ダウンロードは出品者と購入者だけ、エラーの HTTP への変換と内部情報を隠すこと。

## 補助部品

`internal/testutil`(各サービスに同じものがあります)は、テストでだけ使います。

- `NewDB(t, database.AutoMigrate)`: 上の専用スキーマを作って、接続した `*gorm.DB` を返します。
- `NewFakeDB(t)`: Postgres なしで作れる、`Ping` の成否だけを切り替えられる偽の DB です(ヘルスチェックのテスト用)。

## Windows でテスト用バイナリが実行できないとき

環境によっては、「アプリケーション制御ポリシーによってこのファイルがブロックされました」と出て、`go test` が失敗することがあります。テストの失敗ではなく、一時フォルダに作られたテスト用バイナリが OS に止められています。別の場所にビルドして実行すると通ることがあります。

```bash
go test -c -o ./out.test.exe ./internal/repository
cd internal/repository && ../../out.test.exe -test.v
```
