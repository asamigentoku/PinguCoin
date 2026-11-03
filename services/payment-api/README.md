# payment-api

## 現在の決済フロー

`inventory.reserved.v1` を受信した Kafka consumer は、gRPC handler ではなく
`PaymentUseCase.Create` を直接呼び出す。決済状態と `outbox_events` は同じ DB
transaction で保存され、別の outbox publisher が `payments.succeeded.v1` または
`payments.failed.v1` を Kafka へ at-least-once 配送する。

- `point`: ポイント減算、決済、outbox を1つのDB transactionで確定する。
- `card` / `credit_card`: `processing` を先に保存し、mock Stripeから返された
  3DS URLを `payments.action-required.v1` で注文へ通知する。署名付きWebhookを
  受信してから `succeeded` とoutboxを確定する。
- mock Stripeにも注文の `idempotency_key` を渡すため、再試行で二重課金しない。
- provider成功後にpayment-apiが停止した場合はreconcilerが `processing` を回収し、
  同じ冪等キーでproviderとローカル状態を収束させる。

Docker Composeではmock Stripeも自動起動する。単体で動かす場合は別ターミナルで起動する。

```bash
go run ./services/payment-api/cmd/mock-stripe
```

payment-apiは `MOCK_STRIPE_URL`（デフォルト `http://localhost:8090`）へ接続する。
mock Stripeは `POST /webhook` 相当のpayment-api webhook port（既定8086）へ通知し、
`WEBHOOK_SECRET` のHMAC署名で偽装イベントを拒否する。provider event IDはDBで一意に
記録するためWebhookの再送でもポイント付与・注文確定は一度だけ行われる。

決済ドメインを扱うAPI。Go + [gRPC](https://grpc.io/) + [GORM](https://gorm.io/) (PostgreSQL) 構成。
proto定義は[buf](https://buf.build/)で管理し、このサービスの `proto/payment/v1` に置く(生成したGoのコードも同じ場所)。呼び出し側の `bff` も、この生成コードをそのまま import する。

## 機能

| 機能 | 規格(RPC) | フロー |
| --- | --- | --- |
| 決済処理 | `PaymentService.CreatePayment` | `point` はDB内で即時確定し、カードは3DS/redirectとWebhookで非同期確定する。`purpose=point_topup` は確定とポイント付与を同じtransactionで行う。Kafka経由の結果はtransactional outboxから配送する |
| 履歴取得 | `PaymentService.GetPayment` / `ListPayments`(`user_id`で絞り込み) / `ListRefunds`(`payment_id`で絞り込み) | 決済・返金の記録をそのまま返す(参照のみ、副作用なし) |
| キャンセル | `PaymentService.CancelPayment` | `status: pending`の決済のみキャンセル可。それ以外は`FAILED_PRECONDITION` |
| 返金 | `PaymentService.RefundPayment` | `status: succeeded`/`partially_refunded`の決済に対して`amount`を指定して返金。累計返金額が決済額を超える場合は`INVALID_ARGUMENT`。全額一致で`status: refunded`、一部なら`status: partially_refunded`に更新 |

## ポイント管理(PinguCoin)

| 機能 | 規格(RPC) | フロー |
| --- | --- | --- |
| 残高管理(user_idごと) | `PointService.GetPointAccount` | 取引が1件もないユーザーはレコードなしでも残高0として返す |
| 履歴取得 | `PointService.ListPointTransactions`(`user_id`で絞り込み) | 付与・消費の全履歴(`type`で用途を区別) |
| 手動付与・消費 | `PointService.CreditPoints` / `DebitPoints` | キャンペーン付与など決済に紐づかない増減。`DebitPoints`は残高不足で`FAILED_PRECONDITION` |
| 決済との自動連携 | `PaymentService.CreatePayment` / `RefundPayment`(`payment_method: "point"`時) | `db.Transaction`で決済とポイント増減を1つのトランザクションにまとめて原子的に処理する。残高不足なら決済ごとロールバックされ`FAILED_PRECONDITION` |

## リソース

| テーブル | 説明 |
| --- | --- |
| `payments` | 決済(`user_id`=購入者, `product_id`=media-apiの`products.id`への参照) |
| `refunds` | 返金(1決済に対して複数件になり得る = 部分返金対応) |
| `point_accounts` | ユーザーごとのポイント残高(1ユーザー1件) |
| `point_transactions` | ポイント増減履歴(`type`: `credit`/`debit`/`payment`/`refund`) |

決済ステータス(`payments.status`): `pending` / `succeeded` / `failed` / `canceled` / `refunded` / `partially_refunded`
返金ステータス(`refunds.status`): `pending` / `succeeded` / `failed`

## エラーレスポンス / ログ

media-apiと同じ方針。`internal/apperr`でアプリ固有のエラー(`NotFound`/`InvalidArgument`/`FailedPrecondition`/`Internal`)のみを返し、
DBの生エラーはクライアントに漏らさずログにのみ残す。`internal/interceptor.Logging`で全RPCを構造化ログ(JSON)に出力する。

## 起動確認

```bash
grpcurl -plaintext localhost:8081 list
grpcurl -plaintext -d '{"user_id": 1, "product_id": 1, "amount": 3000, "currency": "JPY", "payment_method": "credit_card"}' \
  localhost:8081 payment.v1.PaymentService/CreatePayment
```

## proto生成

```bash
buf lint
buf generate services/payment-api/proto --template services/payment-api/proto/buf.gen.yaml
```

リポジトリのルートで実行する(`make proto` でも同じ)。生成物は `.proto` と同じ `services/payment-api/proto/payment/v1` に出力される(コミット対象)。

## 開発

```bash
cp .env.example .env
go run ./cmd/api
```

`DB_NAME`のデータベース(デフォルト`payment`)は事前に作成しておく必要がある
(`media-api`と同じPostgresインスタンスを使う場合、`orcan`用に自動作成される`orcan`とは別に作成が必要)。
起動時に `internal/database.Migrate` が、未適用の SQL マイグレーション(`migrations/`。テーブルは `payments`, `refunds`, `point_accounts`, `point_transactions`)を番号順に適用する(仕組みは `docs/VERSIONING.md`)。
