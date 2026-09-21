# orcan-api

商品(EC)ドメインを扱うAPI。Go + [gRPC](https://grpc.io/) + [GORM](https://gorm.io/) (PostgreSQL) 構成。
proto定義は[buf](https://buf.build/)で管理し、リポジトリルートの `proto/orcan/v1` に置く。

## リソース

| テーブル | 説明 |
| --- | --- |
| `products` | 商品(`user_id`=出品者, `image_url`=メイン画像のblob URL) |
| `product_categories` | 商品カテゴリ |
| `product_detail` | 商品画像・詳細 |
| `product_inventory` | 在庫 |
| `product_listings` | 出品情報 |
| `users` | ユーザーのアプリ内プロフィール(`clerk_user_id`でClerkのユーザーと1:1対応。パスワード等の認証情報は一切保持しない) |

## サービス

各リソースに対して標準的なCRUDのgRPCサービスを提供する(定義: `proto/orcan/v1/*.proto`)。

- `orcan.v1.ProductService`: `ListProducts`(`user_id`で絞り込み可), `GetProduct`, `CreateProduct`, `UpdateProduct`, `DeleteProduct`
- `orcan.v1.ProductCategoryService`: `ListProductCategories`, `GetProductCategory`, `CreateProductCategory`, `UpdateProductCategory`, `DeleteProductCategory`
- `orcan.v1.ProductDetailService`: `ListProductDetails`(`product_id`で絞り込み可), `GetProductDetail`, `CreateProductDetail`, `UpdateProductDetail`, `DeleteProductDetail`
- `orcan.v1.ProductInventoryService`: `ListProductInventories`, `GetProductInventory`, `CreateProductInventory`, `UpdateProductInventory`, `DeleteProductInventory`
- `orcan.v1.ProductListingService`: `ListProductListings`(`product_id`で絞り込み可), `GetProductListing`, `CreateProductListing`, `UpdateProductListing`, `DeleteProductListing`
- `orcan.v1.UserService`: `ListUsers`, `GetUser`, `GetUserByClerkID`, `EnsureUser`(clerk_user_idで取得、無ければ作成。存在確認/新規作成のJITプロビジョニング用),
  `UpdateUser`(氏名のみ), `DeleteUser`

認証はClerk(フロントエンド)が担う。orcan-apiはログイン処理やトークン発行を一切行わず、
`pingu-api`がClerkのセッショントークンを検証した上で`EnsureUser`/`GetUserByClerkID`を呼び出し、
アプリ内のユーザープロフィール(`users`テーブル)と紐づける。

サーバー起動時に [gRPC reflection](https://pkg.go.dev/google.golang.org/grpc/reflection) を有効化しているため、
[grpcurl](https://github.com/fullstorydev/grpcurl) 等でスキーマなしに疎通確認できる。

```bash
grpcurl -plaintext localhost:8080 list
grpcurl -plaintext localhost:8080 orcan.v1.ProductService/ListProducts
```

## proto生成

proto定義を変更したら `proto/` ディレクトリで生成し直す。

```bash
cd proto
buf lint
buf generate --template buf.gen.orcan.yaml --path orcan
```

生成物は `server/orcan-api/internal/pb/orcan/v1` に出力される(コミット対象)。

## エラーレスポンス

`internal/apperr` で定義したアプリ固有のエラーだけを返す(DBの生エラーはクライアントに漏らさない)。

- gRPCの`codes.Code`(`NotFound`/`InvalidArgument`/`Internal`)に加えて、
  `google.rpc.ErrorInfo`として機械可読な`reason`(例: `"NOT_FOUND"`)をエラー詳細に付与する。
- `apperr.NotFound(resource, err)` / `apperr.InvalidArgument(msg)` / `apperr.Internal(err)` を
  各ハンドラー(`internal/grpcserver`)から返すだけでよい。元のDBエラー(`err`)はログにのみ使われる。

## ログ

`log/slog`によるJSON構造化ログを標準出力に出す。

- `internal/interceptor.Logging` がgRPCの全RPCを自動でログする(メソッド名・処理時間・結果コード)。
  各ハンドラー側で個別にログを書く必要はない。成功はInfo、`NotFound`等クライアント起因のエラーはWarn、
  `Internal`はErrorで元のエラー内容まで出す。
- GORMのクエリログも`internal/database.NewGormLogger`でslogに統一している
  (`gorm.ErrRecordNotFound`は正常系として扱うためログには出さない)。

## 開発

```bash
cp .env.example .env
go run ./cmd/api
```

起動時に `internal/database.AutoMigrate` が全テーブルのマイグレーションを実行する。
