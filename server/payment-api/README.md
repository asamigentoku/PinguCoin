# payment-api

決済ドメインを扱うAPI。Go + [gRPC](https://grpc.io/) + [GORM](https://gorm.io/) (PostgreSQL) 構成。
proto定義は[buf](https://buf.build/)で管理し、リポジトリルートの `proto/payment/v1` に置く。

## 機能

| 機能 | 規格(RPC) | フロー |
| --- | --- | --- |
| 決済処理 | `PaymentService.CreatePayment` | リクエストを受けてその場で処理し `status: succeeded` の`Payment`を返す(現状は外部決済ゲートウェイ連携なし。将来カード決済等を繋ぐ場合は`status: pending`で返し、Webhookなどで確定させる形に拡張する想定) |
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
| `payments` | 決済(`user_id`=購入者, `product_id`=orcan-apiの`products.id`への参照) |
| `refunds` | 返金(1決済に対して複数件になり得る = 部分返金対応) |
| `point_accounts` | ユーザーごとのポイント残高(1ユーザー1件) |
| `point_transactions` | ポイント増減履歴(`type`: `credit`/`debit`/`payment`/`refund`) |

決済ステータス(`payments.status`): `pending` / `succeeded` / `failed` / `canceled` / `refunded` / `partially_refunded`
返金ステータス(`refunds.status`): `pending` / `succeeded` / `failed`

## エラーレスポンス / ログ

orcan-apiと同じ方針。`internal/apperr`でアプリ固有のエラー(`NotFound`/`InvalidArgument`/`FailedPrecondition`/`Internal`)のみを返し、
DBの生エラーはクライアントに漏らさずログにのみ残す。`internal/interceptor.Logging`で全RPCを構造化ログ(JSON)に出力する。

## 起動確認

```bash
grpcurl -plaintext localhost:8081 list
grpcurl -plaintext -d '{"user_id": 1, "product_id": 1, "amount": 3000, "currency": "JPY", "payment_method": "credit_card"}' \
  localhost:8081 payment.v1.PaymentService/CreatePayment
```

## proto生成

```bash
cd proto
buf lint
buf generate --template buf.gen.payment.yaml --path payment
```

生成物は `server/payment-api/internal/pb/payment/v1` に出力される(コミット対象)。

## 開発

```bash
cp .env.example .env
go run ./cmd/api
```

`DB_NAME`のデータベース(デフォルト`payment`)は事前に作成しておく必要がある
(`orcan-api`と同じPostgresインスタンスを使う場合、`orcan`用に自動作成される`orcan`とは別に作成が必要)。
起動時に `internal/database.AutoMigrate` がテーブル(`payments`, `refunds`, `point_accounts`, `point_transactions`)のマイグレーションを実行する。
