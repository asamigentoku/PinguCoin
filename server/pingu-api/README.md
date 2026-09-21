# pingu-api

クライアント(admin-web/client-web)向けのAPIゲートウェイ。Go構成。
`orcan-api`(商品・ユーザー・認証)と`payment-api`(決済)へgRPCで委譲し、
GraphQL/RESTFulとして統合して外部へ公開する。

## 機能一覧

| 内容 | 規格 | フロー |
| --- | --- | --- |
| 商品登録、閲覧、編集、削除 | GraphQL(`/graphql`) | → orcan-api(`ProductService`, DB反映) |
| 一般ログイン、ログアウト | フロントエンド(Clerk)で完結 | フロントがClerkでログイン/ログアウトし、以降のリクエストにClerkのセッショントークンを添える |
| 商品購入(注文API) | RESTful(`POST /orders`, `GET /orders`, `GET /orders/{id}`) | → orcan-api(商品確認) → payment-api(決済) → pingu-api自身のDBに注文(Order)として記録 |
| ユーザー情報、登録、編集 | GraphQL(`/graphql`) | → orcan-api(`UserService`, DB反映)。登録はClerk側で行われ、初回アクセス時にpingu-apiが自動でプロフィールを作成する |

`orcan-api`/`payment-api`は内部向けの薄いCRUD/決済APIで、複数サービスをまたぐ
オーケストレーション(注文の作成、Clerkユーザーとアプリ内プロフィールの紐付けなど)はすべてこの`pingu-api`が担う。

## アーキテクチャ

- 認証は**Clerk**(フロントエンド)が担う。orcan-api/pingu-apiはパスワード等の認証情報を一切保持しない。
- 商品(Product)・ユーザー(User)は`orcan-api`が真実の記録を持つ。pingu-apiはこれらのDBを持たず、
  GraphQLのresolverから`internal/orcanclient`経由でgRPC呼び出しするだけ。
- 決済(Payment)は`payment-api`が真実の記録を持つ。
- 注文(Order)は決済(Payment)とは別エンティティとして、pingu-api自身のDB(`orders`テーブル)に保持する
  (`payment_id`でpayment-apiのPaymentを参照)。「購入する」という行為の記録と、決済処理そのものの記録を分離するため。

## 認証(Clerk)

- ログイン/ログアウトはフロントエンド(Next.js + Clerk SDK)側で完結する。pingu-apiにRESTの
  `/auth/login`・`/auth/logout`は存在しない。
- フロントは各リクエストに`Authorization: Bearer <Clerkのセッショントークン>`ヘッダを付けて呼び出す。
- pingu-apiは`internal/clerkauth`でトークンをClerkのJWKS(公開鍵)により検証する
  (`CLERK_SECRET_KEY`はJWKS取得・ユーザー情報取得のためのBackend API呼び出しに使う。
  トークンの署名自体はClerk側の秘密鍵で行われるため、pingu-apiは署名用の秘密鍵を持たない)。
- 検証したClerkのユーザーID(`sub`)を使い、`orcan-api`の`UserService.GetUserByClerkID`で
  アプリ内プロフィールを解決する。まだ存在しない(初回アクセス)場合のみClerkのBackend APIで
  email/氏名を取得し、`UserService.EnsureUser`でプロフィールを作成する(JITプロビジョニング)。
  2回目以降はorcan-api側のDB参照のみで完結し、Clerk APIへの追加呼び出しは発生しない。
- GraphQLの`me`/`updateUser`、RESTの`/orders`系はログイン必須。それ以外の商品CRUD・ユーザー参照は
  現状未認証でも呼び出せる(薄いゲートウェイとしての最小実装)。

## proto生成

`orcan-api`/`payment-api`のクライアントスタブ(gRPC)を、リポジトリルートの`proto/`から生成する。

```bash
cd proto
buf lint
buf generate --template buf.gen.pingu.yaml --path orcan --path payment
```

生成物は`server/pingu-api/internal/pb/{orcan,payment}/v1`に出力される(コミット対象)。

## GraphQL

[gqlgen](https://gqlgen.com/)を使用。スキーマは`graph/schema.graphqls`。
スキーマを変更したら以下で再生成する(resolverの実装は保持される)。

```bash
go run github.com/99designs/gqlgen generate
```

## 開発

```bash
cp .env.example .env
go run ./cmd/api
```

`DB_NAME`のデータベース(デフォルト`pingu`)は事前に作成しておく必要がある
(`orcan-api`/`payment-api`と同じPostgresインスタンスを使う場合、別途作成が必要)。
起動時に`internal/database.AutoMigrate`が`orders`テーブルのマイグレーションを実行する。

起動後、`http://localhost:8082/`でGraphQL Playgroundを確認できる。
`orcan-api`(デフォルト`localhost:8080`)と`payment-api`(デフォルト`localhost:8081`)を
先に起動しておくこと。
