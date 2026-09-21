# 通信規格(API仕様)

各サーバー間・クライアント間の通信方式と、各APIの処理フローをまとめたもの。テーブル構造は [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) を参照。

> **注記(2026-09-21)**: このリポジトリは現在、認証方式を orcan-api 独自のJWT(メール/パスワード)から **Clerk**(フロントエンド側で認証するSaaS)へ移行する作業の途中。`proto`・`model`・GraphQLスキーマ側はすでにClerk前提の形に変わっているが、pingu-apiのHTTP層(ルーティング/ミドルウェア/JWT検証)はまだ旧実装のまま残っており、**現状ビルドが通らない不整合な状態**。本ドキュメントは「実装済みで動く部分」と「移行未完了の部分」を区別して記載する。

## 全体構成

```
[クライアント]
     │  GraphQL (POST /graphql) ── 商品・ユーザーの参照/更新(薄いゲートウェイ)
     │  REST    (POST/GET /auth/*, /orders*) ── ログイン・注文
     ▼
[pingu-api] ──gRPC──▶ [orcan-api]   (商品・ユーザーの真実の記録)
     │            └──▶ [payment-api] (決済・返金・ポイントの真実の記録)
     └─ Order を自身のDBに保持(payment_id/product_id/user_idはID参照のみ)
```

- orcan-api・payment-api は **gRPC専用サーバー**(HTTPは公開しない)。
- pingu-api は **GraphQL + REST のゲートウェイ**として、クライアントからのHTTPリクエストをgRPCに変換して orcan-api / payment-api へ委譲する。
- サービス間はすべてgRPC。DBを跨いだ外部キー制約は張らず、IDのみを参照する。

### サービスアドレス(デフォルト値、環境変数で上書き可)

| サービス | プロトコル | デフォルトポート | 環境変数 |
|---|---|---|---|
| orcan-api | gRPC | `:8080` | `PORT` |
| payment-api | gRPC | `:8081` | `PORT` |
| pingu-api | HTTP(GraphQL/REST) | `:8082` | `PORT` |
| pingu-api → orcan-api | gRPC接続先 | `localhost:8080` | `ORCAN_API_ADDR` |
| pingu-api → payment-api | gRPC接続先 | `localhost:8081` | `PAYMENT_API_ADDR` |

いずれのgRPCサーバーも `grpc.ChainUnaryInterceptor` でアクセスログを出力し、`reflection.Register` によりgrpcurl等からスキーマを問い合わせ可能。

### 全RPC/エンドポイント共通のエラーハンドリング

- gRPC層: `internal/apperr` によりエラー種別(`Reason`: InvalidArgument/NotFound/AlreadyExists/FailedPrecondition/Internal 等)を持つ構造化エラーを返す。
- pingu-apiのGraphQL層: gRPCエラーを`apperr.FromGRPC`で変換し、`reason` extensionを付与。内部エラー(Internal)の詳細はクライアントに返さず、サーバーログにのみ出力する。
- REST層も同様に`apperr`ベースで統一されたエラーレスポンスを返す。

---

## orcan-api(gRPC)

商品・ユーザー(アプリ内プロフィール)を扱う。proto: `proto/orcan/v1/*.proto`
**旧`AuthService`(メール/パスワードログイン)は削除済み** — 認証はClerk(フロントエンド)側の責務になり、orcan-apiはパスワード等の認証情報を一切保持しない。

### UserService(`user.proto`)

| RPC | Request | Response | 概要 |
|---|---|---|---|
| `ListUsers` | `{}` | `{users[]}` | |
| `GetUser` | `{id}` | `{user}` | |
| `GetUserByClerkID` | `{clerk_user_id}` | `{user}` | Clerkユーザーからアプリ内プロフィールを引く |
| `EnsureUser` | `{clerk_user_id, email, name}` | `{user}` | なければ作成(JITプロビジョニング) |
| `UpdateUser` | `{id, name}` | `{user}` | 氏名の更新のみ |
| `DeleteUser` | `{id}` | `{}` | |

#### 処理フロー: `EnsureUser`(clerk_user_idからユーザーを解決/自動作成)

pingu-apiがClerkのセッショントークンを検証した後、初回アクセス時のプロビジョニング用に呼ぶことを想定したRPC(現状、実際に呼び出す側の実装はまだ無い。「認証フロー」の項を参照)。

```
1. clerk_user_id が空 → InvalidArgument
2. repo.FindByClerkUserID(clerk_user_id) で既存ユーザーを検索
   a. 見つかった → そのユーザーをそのまま返す(email/nameは上書きしない)
   b. gorm.ErrRecordNotFound → 3へ
   c. それ以外のDBエラー → Internal
3. model.User{ClerkUserID, Email, Name} を作成し保存
4. 作成したユーザーをレスポンスとして返す
```

#### 処理フロー: 汎用CRUD(`ListUsers`/`GetUser`/`UpdateUser`/`DeleteUser`、および ProductService / ProductCategoryService / ProductDetailService / ProductInventoryService / ProductListingService の全RPC)

orcan-apiの大半のRPCは同一パターン。個別の分岐は上表・各protoのフィールド制約(必須項目・非負制約など)のみ。

```
1. リクエストのバリデーション(必須フィールド・値域チェック) → 不正なら InvalidArgument
2. (Update/Delete/Getの場合) repo.FindByID でレコード取得
   → 見つからなければ NotFound
3. repository経由でDB操作(Create/Update/Delete)
   → DBエラーなら Internal
4. DBのmodelをprotoメッセージ(pb.Xxx)に変換して返す
```

代表例(`CreateProduct`):
```
クライアント ──CreateProductRequest──▶ ProductServer.CreateProduct
   1) validateProductInput(name, user_id, category_id) が空/0ならInvalidArgument
   2) model.Product を組み立てて repo.Create(product) (INSERT INTO products)
   3) toProtoProduct(product) に変換して CreateProductResponse を返す
```

---

## payment-api(gRPC)

決済・返金・ポイントを扱う。proto: `proto/payment/v1/*.proto`

### PaymentService(`payment.proto`)

| RPC | Request | Response | 概要 |
|---|---|---|---|
| `CreatePayment` | `{user_id, product_id, amount, currency, payment_method}` | `{payment}` | 決済処理 |
| `GetPayment` | `{id}` | `{payment}` | |
| `ListPayments` | `{user_id?}` | `{payments[]}` | |
| `ListRefunds` | `{payment_id?}` | `{refunds[]}` | |
| `CancelPayment` | `{id, reason}` | `{payment}` | 決済確定前(pending)のキャンセル |
| `RefundPayment` | `{payment_id, amount, reason}` | `{refund, payment}` | 確定済み決済への返金(部分返金対応) |

#### 処理フロー: `CreatePayment`

外部決済ゲートウェイとの連携は未実装で、リクエストが来た時点でその場を成功(`succeeded`)として処理する(将来カード決済等を繋ぐ場合は`pending`のままWebhookで確定する形に拡張予定)。`payment_method="point"`の場合のみ、決済とポイント消費を1トランザクションにまとめて原子的に処理する。

```
1. user_id / product_id / amount(>0) / currency / payment_method のバリデーション
   → 不正なら InvalidArgument
2. model.Payment{Status: "succeeded"} を組み立てる

3. payment_method == "point" の場合:
   db.Transaction 開始
     a. payments.Create(payment)                              (INSERT INTO payments)
     b. points.Adjust(user_id, -amount, "payment", "payment #<id>", paymentID)
        - point_accounts を SELECT ... FOR UPDATE で行ロック
        - 残高不足(newBalance < 0) なら ErrInsufficientPoints でロールバック
          → FailedPrecondition("insufficient points")
        - point_accounts.balance を更新                        (UPDATE point_accounts)
        - point_transactions に type="payment" の履歴を1件作成   (INSERT INTO point_transactions)
   トランザクション終了(全て成功 or 全てロールバック)

   それ以外の payment_method の場合:
   a. payments.Create(payment) のみ                            (INSERT INTO payments)

4. toProtoPayment(payment) に変換して CreatePaymentResponse を返す
```

#### 処理フロー: `RefundPayment`

```
1. payment_id(必須) / amount(>0) のバリデーション → 不正なら InvalidArgument
2. payments.FindByID(payment_id) で対象決済を取得 → 無ければ NotFound
3. payment.Status が succeeded/partially_refunded 以外 → FailedPrecondition
4. refunds.SumSucceededAmount(payment_id) で既返金額を集計
5. remaining = payment.Amount - 既返金額
   amount > remaining → InvalidArgument("refund amount exceeds refundable amount")
6. newStatus を決定(amount == remaining なら "refunded"、そうでなければ "partially_refunded")

7. payment_method == "point" の場合:
   db.Transaction 開始
     a. refunds.Create(refund)                                 (INSERT INTO refunds)
     b. payment.Status = newStatus; payments.Update(payment)    (UPDATE payments)
     c. points.Adjust(user_id, +amount, "refund", "refund #<id>", paymentID)
        - 残高に amount を加算し、point_transactions に type="refund" の履歴を1件作成
   トランザクション終了

   それ以外の場合:
   a. refunds.Create(refund)                                   (INSERT INTO refunds)
   b. payment.Status = newStatus; payments.Update(payment)      (UPDATE payments)

8. refund・payment(更新後)を RefundPaymentResponse として返す
```

#### 処理フロー: `CancelPayment`

```
1. payments.FindByID(id) → 無ければ NotFound
2. payment.Status != "pending" → FailedPrecondition("payment is not cancellable in status: ...")
3. payment.Status = "canceled"; payments.Update(payment)        (UPDATE payments)
4. 更新後の payment を返す
```
現状 `CreatePayment` は即座に `succeeded` を返すため、実運用でこのRPCが通るケース(pending状態の決済)は無い。非同期決済ゲートウェイ導入後に活きる想定。

### PointService(`point.proto`)

| RPC | Request | Response | 概要 |
|---|---|---|---|
| `GetPointAccount` | `{user_id}` | `{account}` | 残高取得 |
| `ListPointTransactions` | `{user_id?}` | `{transactions[]}` | 履歴取得 |
| `CreditPoints` | `{user_id, amount, reason}` | `{transaction, account}` | 手動付与(キャンペーン等) |
| `DebitPoints` | `{user_id, amount, reason}` | `{transaction, account}` | 手動消費 |

#### 処理フロー: `GetPointAccount`

```
1. user_id(必須) のバリデーション → 0ならInvalidArgument
2. points.GetAccount(user_id) で point_accounts を検索
   - gorm.ErrRecordNotFound(一度も取引がないユーザー) → エラーにせず
     残高0の PointAccount{UserID: user_id, Balance: 0} を即席で組み立てる
   - それ以外のエラー → Internal
3. toProtoPointAccount で変換して返す
```

#### 処理フロー: `CreditPoints` / `DebitPoints`

`PaymentService.CreatePayment`のポイント消費ロジックとは別経路(決済に紐づかない手動操作)。`points.AdjustAtomic`が自前でトランザクションを張る。

```
1. user_id(必須) / amount(>0) / reason(必須) のバリデーション → 不正ならInvalidArgument
2. AdjustAtomic(user_id, ±amount, "credit"|"debit", reason, paymentID=nil)
   db.Transaction 開始
     a. point_accounts を SELECT ... FOR UPDATE で行ロック(無ければ balance=0 で新規作成)
     b. newBalance = balance ± amount
        DebitPoints で newBalance < 0 → ErrInsufficientPoints
          → FailedPrecondition("insufficient points")
     c. point_accounts.balance を更新                           (UPDATE point_accounts)
     d. point_transactions に type="credit"|"debit" の履歴を1件作成 (INSERT INTO point_transactions)
   トランザクション終了
3. transaction・account を レスポンスとして返す
```

---

## pingu-api(GraphQL + REST ゲートウェイ)

クライアント向けにHTTPを公開し、内部でorcan-api/payment-apiへgRPCで委譲する。

### エンドポイント一覧

| メソッド | パス | 種別 | 実装状態 |
|---|---|---|---|
| GET | `/` | - | GraphQL Playground(開発用) |
| POST | `/graphql` | GraphQL | 動作する(商品・ユーザーのCRUD) |
| POST | `/auth/login` | REST | **旧JWT実装のまま。`orcan-api`に既に存在しない`AuthService`を呼んでおり現状動かない** |
| POST | `/auth/logout` | REST | Cookie破棄のみなので単体では動くが、意味を持たなくなっている(下記「認証フロー」参照) |
| POST | `/orders` | REST | 動作する(商品購入) |
| GET | `/orders` | REST | 動作する(自分の注文一覧) |
| GET | `/orders/{id}` | REST | 動作する(注文詳細) |

全リクエストは `WithLogging` → `WithOptionalAuth` の順にミドルウェアを通ってから各ハンドラ/GraphQLサーバーに到達する。

### 処理フロー: GraphQLリクエスト全体(`POST /graphql`)

```
クライアント ──POST /graphql (query/mutation)──▶ pingu-api
  1. WithLogging: アクセスログ出力
  2. WithOptionalAuth: Authorizationヘッダ or Cookieのトークンがあれば検証し、
     成功時のみ context にログインユーザー(reqcontext.Claims)を積む
     (トークン無し/不正でもリクエスト自体は通す)
  3. gqlgenが query/mutation をパースし、対応する resolver を呼ぶ
  4. 各resolver:
       - r.Orcan.Product.* / r.Orcan.User.* を呼び、orcan-apiへgRPCで委譲
       - 認証が必須なresolver(me, updateUser)は reqcontext.UserFromContext で
         ログイン確認 → 未ログインなら Unauthenticated
       - updateUser はさらに「本人のみ更新可」を claims.UserID == id で確認
  5. gRPCエラーは apperr.FromGRPC で変換 → newErrorPresenter が
     GraphQLエラー(reason extension付き)に整形してクライアントへ返す
  6. 成功時は pb.* を graph/convert.go の *FromPB で GraphQLモデルに変換して返す
```

代表例(`mutation createProduct`):
```
クライアント ──createProduct(input)──▶ mutationResolver.CreateProduct
   → orcanclient経由で orcan-api.ProductService.CreateProduct を呼ぶ(gRPC)
   → orcan-api: バリデーション → products テーブルにINSERT
   → 結果(pb.Product)を model.Product に変換して返す
```

### 処理フロー: `POST /orders`(商品購入)

3サービスにまたがる最も複雑な処理。**要ログイン**。

```
クライアント ──POST /orders {product_id, quantity, payment_method}──▶ OrderHandler.CreateOrder

1. reqcontext.UserFromContext でログイン確認 → 未ログインなら Unauthenticated
2. リクエストボディをデコード。product_id必須、quantity未指定は1、
   payment_method未指定は"point"をデフォルトに補完
3. orcan-api.ProductService.GetProduct(product_id) を呼び、商品の現在価格を取得(gRPC)
   → 見つからなければ orcan-api由来のNotFoundがそのまま返る
4. totalAmount = product.price * quantity を算出
5. payment-api.PaymentService.CreatePayment(user_id, product_id, totalAmount,
   currency="JPY", payment_method) を呼ぶ(gRPC)
   → payment-api側で決済作成(payment_method="point"ならポイント残高も同時に消費。
     残高不足なら FailedPrecondition("insufficient points") がそのまま返る)
6. 決済結果(payment.status)から注文ステータスを導出:
     "succeeded" → "paid" / "pending" → "pending" / それ以外 → "failed"
7. model.Order{user_id, product_id, quantity, unit_price, total_amount,
   payment_id, status} を組み立て、pingu-api自身のDBに保存 (INSERT INTO orders)
8. 201 Created で注文情報をJSONで返す
```

呼び出し順序上の注意: 3→4→5→7 のいずれかで失敗しても、**それより前のステップで確定した副作用(決済の作成等)は取り消されない**(payment-api内の決済作成とポイント消費は原子的だが、「決済は成功したのに注文がDBに保存されない」ケースはあり得る)。

### 処理フロー: `GET /orders` / `GET /orders/{id}`

```
1. ログイン確認(Unauthenticated)
2. (詳細取得の場合) パスパラメータをuintにパース → 不正なら InvalidArgument
3. repo経由で orders テーブルから取得
   - GetOrder: 見つからない、または order.user_id != ログインユーザー → NotFound
     (他ユーザーの注文の存在を推測されないよう、権限エラーではなく404として扱う)
4. JSONで返す
```

### 認証フロー(移行中 — 現状動く実装 と 目標実装の両方を記載)

#### (A) 現状コードに残っている実装(旧: orcan-api発行のJWT)— **現状ビルド不可**

```
POST /auth/login {email, password}
  → AuthHandler.Login が orcan.Auth.Login(email, password) を呼ぶ(gRPC)
  → ただし orcan-api には既に AuthService が存在しない(auth.proto ごと削除済み)ため、
    pingu-api の orcanclient.Client.Auth(pb.AuthServiceClient) は生成コード自体が無く、
    コンパイルが通らない
  (成功していた頃の想定フロー: orcan-apiでbcrypt照合 → JWT発行 → httpOnly Cookie(pingu_token)
   にセット、レスポンスbodyにもtoken返却)

各リクエスト共通:
  WithOptionalAuth ミドルウェアが Authorization: Bearer <token> または
  Cookie(pingu_token) から authtoken.Verify(token, JWT_SECRET) でHS256検証し、
  成功すればcontextにClaims{UserID, Email, Name}を積む
```

#### (B) スキーマ・モデル・コメントが指し示す目標実装(Clerkベース)— **未実装**

`schema.graphqls`・`reqcontext.Claims`・orcan-apiの`EnsureUser`は、以下の想定で既に用意されているが、pingu-api側でこれを実行するミドルウェア/ハンドラはまだ書かれていない。

```
1. クライアントはフロントエンド(Clerk SDK)でログインし、Clerkのセッショントークンを取得
2. pingu-apiへのリクエストに Authorization ヘッダでそのトークンを付与
3. pingu-api側ミドルウェア(未実装)がClerkのトークンを検証し、
   clerk_user_id / email / name を取り出す
4. orcan-api.UserService.EnsureUser(clerk_user_id, email, name) を呼び、
   アプリ内プロフィール(users テーブルの行)が無ければ自動作成(JITプロビジョニング)
5. 解決した users.id を reqcontext.Claims{UserID, ClerkUserID, Email, Name} として
   contextに積み、以降 me / updateUser / /orders* などの認証必須処理から参照する
```

`POST /auth/login`・`POST /auth/logout`・`JWT_SECRET`設定・`authtoken`パッケージは、
この移行が完了すれば不要になり削除される見込み(Clerk側がログインUIとセッション管理を担うため)。
