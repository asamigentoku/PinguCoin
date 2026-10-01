# orcan-api

商品(EC)ドメインを扱うAPI。Go + [gRPC](https://grpc.io/) + [GORM](https://gorm.io/) (PostgreSQL) 構成。
proto定義は[buf](https://buf.build/)で管理し、このサービスの `proto/orcan/v1` に置く(生成したGoのコードも同じ場所)。呼び出し側の `pingu-api` も、この生成コードをそのまま import する。

## リソース

| テーブル | 説明 |
| --- | --- |
| `products` | 商品(`user_id`=出品者, `image_url`=メイン画像のblob URL, `file_url`=販売対象のデジタルコンテンツのblob URL, `version`=更新のたびに増える値。呼び出し側のキャッシュ無効化判定に使う) |
| `product_asset_purposes` | 保存用途マスタ。1=商品イメージ、2=商品詳細イメージ、3=販売商品ファイル。行追加で将来用途を拡張可能 |
| `product_assets` | 商品に紐づく共通保存データ。用途を問わず商品1件に対して複数件。画像を含むレスポンスでは削除用の`id`も返す |
| `product_categories` | 商品カテゴリ |
| `product_detail` | 商品画像・詳細 |
| `product_inventory` | 在庫(`version`は`products`と同じ目的) |
| `product_inventory_transactions` | 在庫増減履歴(`idempotency_key`で冪等性を担保する台帳。`point_transactions`と同じ考え方) |
| `product_listings` | 出品情報 |
| `users` | ユーザーのアプリ内プロフィール(`clerk_user_id`でClerkのユーザーと1:1対応。パスワード等の認証情報は一切保持しない) |

## サービス

各リソースに対して標準的なCRUDのgRPCサービスを提供する(定義: `services/orcan-api/proto/orcan/v1/*.proto`)。

- `orcan.v1.ProductService`: `ListProducts`(`user_id`で絞り込み可), `GetProduct`, `CreateProduct`, `UpdateProduct`, `DeleteProduct`。
  加えてAzure Blob Storageへの画像/ファイルアップロード・ダウンロード用に以下を提供する(いずれも商品の所有者本人のみ実行可、詳細は`internal/storage`参照):
  - `GetProductImageUploadURL` / `ConfirmProductImageUpload` / `DeleteProductImageUpload`:
    商品画像(`main_image`/`sub_images`)用。公開コンテナに保存するため、アップロード後は署名不要の公開URLでそのまま閲覧できる。
    `main_image`配下は`products.image_url`、`sub_images`配下は`product_detail`の1件として保存する。
  - `GetProductFileUploadURL` / `ConfirmProductFileUpload` / `DeleteProductFileUpload` / `GetProductDownloadURL`:
    販売対象のデジタルコンテンツ(`products.file_url`)用。非公開コンテナに保存し、ダウンロードのたびに
    `GetProductDownloadURL`で発行する署名付きURL(Blob単位、有効期限付き)が必要。
    orcan-apiは購入状況を持たないため、呼び出し元(pingu-api)が権限確認済みであることを前提とする。
- `orcan.v1.ProductAssetService`: 共通保存API。`purpose_id`をDBに保存し、商品イメージ・商品詳細イメージ・販売商品ファイルをすべて1対多で扱う。
  `ListProductAssets`と`ConfirmProductAssetUpload`は各データの`id`を返し、削除はURLではなく`DeleteProductAsset(asset_id)`で行う。
  既存の`products.image_url`、`product_detail`、`products.file_url`は起動時に共通テーブルへ冪等に移行される。
- `orcan.v1.ProductCategoryService`: `ListProductCategories`, `GetProductCategory`, `CreateProductCategory`, `UpdateProductCategory`, `DeleteProductCategory`
- `orcan.v1.ProductDetailService`: `ListProductDetails`(`product_id`で絞り込み可), `GetProductDetail`, `CreateProductDetail`, `UpdateProductDetail`, `DeleteProductDetail`
- `orcan.v1.ProductInventoryService`: `ListProductInventories`, `GetProductInventory`, `CreateProductInventory`, `UpdateProductInventory`, `DeleteProductInventory`。
  加えて`AdjustProductInventory`(在庫数を差分で増減。負=消費/正=戻し)を提供する。`idempotency_key`必須で、
  同じキーの再送は二重に増減しない(`product_inventory_transactions`テーブルに履歴を保持)。
  在庫不足時は`FAILED_PRECONDITION`を返す。
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

proto定義を変更したら、リポジトリのルートで生成し直す(`make proto` でも同じ)。

```bash
buf lint
buf generate services/orcan-api/proto --template services/orcan-api/proto/buf.gen.yaml
```

生成物は `.proto` と同じ `services/orcan-api/proto/orcan/v1` に出力される(コミット対象)。

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

起動時に `internal/database.Migrate` が、未適用の SQL マイグレーション(`migrations/`)を番号順に適用する(仕組みは `docs/VERSIONING.md`)。
