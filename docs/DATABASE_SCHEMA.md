# データベーステーブル構造

各サーバー(orcan-api / payment-api / pingu-api)が保持するテーブル構造をまとめたもの。
全サーバーとも PostgreSQL + GORM の `AutoMigrate` によってテーブルを作成しており、
以下は各 `internal/model/*.go` の定義に基づく実際のスキーマ。

- 各サービスは自分が所有するテーブルのみをマイグレーションする(サービスをまたぐ参照は外部キー制約を張らず、IDのみを保持する)。
- `DeletedAt` があるテーブルは GORM の論理削除(soft delete)対象。

---

## orcan-api

商品・ユーザーを管理する。DB: PostgreSQL(`AutoMigrate` 対象には共通保存データの`ProductAssetPurpose`、`ProductAsset`も含む)

### users

> **2026-09-21時点で移行中**: 認証をorcan-api独自のJWT(メール/パスワード)からClerk(フロントエンド側で認証)に切り替え済み。`password_hash`は廃止され、Clerkのユーザーと1:1に対応する`clerk_user_id`を持つアプリ内プロフィールのみになった。
> GORMの`AutoMigrate`はカラムの追加のみを行い削除はしないため、`database.AutoMigrate`内で`password_hash`列の存在を明示的にチェックし`DropColumn`している(残すとNOT NULL制約により`clerk_user_id`のみでのユーザー作成が失敗するため)。

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| clerk_user_id | string(255) | NOT NULL, UNIQUE INDEX(ClerkのユーザーIDと1:1) |
| email | string(255) | NOT NULL(UNIQUE制約なし。真実の記録はClerk側) |
| name | string(255) | NOT NULL |
| created_at | time.Time | |
| updated_at | time.Time | |
| deleted_at | gorm.DeletedAt | INDEX(論理削除) |

### product_categories

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| parent_id | *uint | INDEX(自己参照, NULL可) |
| name | string(100) | NOT NULL |
| created_at | time.Time | |
| updated_at | time.Time | |
| deleted_at | gorm.DeletedAt | INDEX(論理削除) |

### products

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| user_id | uint | NOT NULL, INDEX |
| category_id | uint | NOT NULL, INDEX(FK: product_categories.id) |
| name | string(255) | NOT NULL |
| description | text | |
| image_url | string(512) | |
| price | int64 | NOT NULL |
| status | string(20) | NOT NULL, DEFAULT 'draft' |
| created_at | time.Time | |
| updated_at | time.Time | |
| deleted_at | gorm.DeletedAt | INDEX(論理削除) |

### product_detail

旧商品詳細画像。新規アップロードは`product_assets`の`purpose_id=2`を使用する。

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| product_id | uint | NOT NULL, INDEX |
| image_url | string(512) | |
| description | text | |
| sort_order | int | NOT NULL, DEFAULT 0 |
| created_at | time.Time | |
| updated_at | time.Time | |

### product_asset_purposes

保存データの用途マスタ。用途追加はスキーマ変更ではなく、このテーブルへの行追加で行う。

| カラム | 型 | 制約 |
|---|---|---|
| id | uint16 | PK, 自動採番なし。1=商品イメージ、2=商品詳細イメージ、3=販売商品ファイル |
| name | string(64) | NOT NULL, UNIQUE INDEX |
| is_public | bool | NOT NULL。公開・非公開Blobコンテナの選択に使用 |
| created_at | time.Time | |
| updated_at | time.Time | |

### product_assets

商品に紐づく画像・PDF等の共通保存データ。すべての商品との関係は1対多。削除APIではこのテーブルの`id`を指定する。

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| product_id | uint | NOT NULL, FK: products.id, INDEX |
| purpose_id | uint16 | NOT NULL, FK: product_asset_purposes.id, INDEX |
| storage_url | string(1024) | NOT NULL, UNIQUE INDEX |
| original_filename | string(255) | |
| content_type | string(255) | |
| file_size | int64 | NOT NULL, DEFAULT 0 |
| description | text | |
| sort_order | int | NOT NULL, DEFAULT 0, INDEX |
| is_primary | bool | NOT NULL, DEFAULT false |
| metadata | jsonb | NOT NULL, DEFAULT `{}` |
| created_at | time.Time | |
| updated_at | time.Time | |

### product_inventory

商品の在庫(1商品につき1件)

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| product_id | uint | NOT NULL, UNIQUE INDEX |
| quantity | int | NOT NULL, DEFAULT 0 |
| reserved | int | NOT NULL, DEFAULT 0 |
| created_at | time.Time | |
| updated_at | time.Time | |

### product_listings

商品の出品情報(1商品に対して複数件の出品が可能)

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| product_id | uint | NOT NULL, INDEX |
| price | int64 | NOT NULL |
| status | string(20) | NOT NULL, DEFAULT 'active' |
| listed_at | time.Time | |
| ended_at | *time.Time | NULL可 |
| created_at | time.Time | |
| updated_at | time.Time | |

---

## payment-api

決済・返金・ポイントを管理する。DB: PostgreSQL(`AutoMigrate` 対象: `Payment`, `Refund`, `PointAccount`, `PointTransaction`)

### payments

1回の決済を表す

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| user_id | uint | NOT NULL, INDEX |
| product_id | uint | NOT NULL, INDEX(orcan-apiの`products.id`をID参照。外部キー制約なし) |
| amount | int64 | NOT NULL |
| currency | string(10) | NOT NULL |
| payment_method | string(30) | NOT NULL(`point`はポイント払い) |
| status | string(20) | NOT NULL, DEFAULT 'pending'(pending/succeeded/failed/canceled/refunded/partially_refunded) |
| created_at | time.Time | |
| updated_at | time.Time | |
| deleted_at | gorm.DeletedAt | INDEX(論理削除) |

### refunds

決済に対する返金(全額 or 一部、1決済に対して複数件になり得る)

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| payment_id | uint | NOT NULL, INDEX(FK: payments.id) |
| amount | int64 | NOT NULL |
| reason | text | |
| status | string(20) | NOT NULL, DEFAULT 'pending'(pending/succeeded/failed) |
| created_at | time.Time | |
| updated_at | time.Time | |

### point_accounts

ユーザーごとのポイント残高(user_idごとに1件)

| カラム | 型 | 制約 |
|---|---|---|
| user_id | uint | PK |
| balance | int64 | NOT NULL, DEFAULT 0 |
| created_at | time.Time | |
| updated_at | time.Time | |

### point_transactions

ポイントの増減履歴(1件が1回の付与/消費に対応)

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| user_id | uint | NOT NULL, INDEX |
| amount | int64 | NOT NULL(正=付与、負=消費) |
| type | string(20) | NOT NULL(credit/debit/payment/refund) |
| payment_id | *uint | INDEX, NULL可(Paymentに紐づく増減の場合のみ設定) |
| reason | text | |
| balance_after | int64 | NOT NULL |
| created_at | time.Time | |

---

## pingu-api

注文(Order)を管理する。商品・ユーザーはorcan-api、決済はpayment-apiが真実の記録を持つため、
pingu-apiは`Order`のみをマイグレーションする。DB: PostgreSQL(`AutoMigrate` 対象: `Order`)

### orders

「誰が・何を・いくつ買ったか」という注文の記録(決済処理そのものはpayment-apiのPaymentが担う)

| カラム | 型 | 制約 |
|---|---|---|
| id | uint | PK |
| user_id | uint | NOT NULL, INDEX |
| product_id | uint | NOT NULL, INDEX(orcan-apiの`products.id`をID参照。外部キー制約なし) |
| quantity | int64 | NOT NULL |
| unit_price | int64 | NOT NULL |
| total_amount | int64 | NOT NULL |
| payment_id | uint | NOT NULL(payment-apiの`payments.id`をID参照。外部キー制約なし) |
| status | string(20) | NOT NULL, DEFAULT 'pending'(pending/paid/failed/canceled) |
| created_at | time.Time | |
| updated_at | time.Time | |

---

## サービス間参照まとめ

サービスをまたぐ関連はすべてID参照のみで、DBレベルの外部キー制約は張られていない。

```
orcan-api:  users, product_categories, products, product_detail,
            product_inventory, product_listings

payment-api: payments(user_id, product_id は他サービスのID参照)
             refunds(payment_id → payments.id)
             point_accounts(user_id)
             point_transactions(user_id, payment_id → payments.id)

pingu-api:  orders(user_id, product_id は orcan-api、
                    payment_id は payment-api のID参照)
```
