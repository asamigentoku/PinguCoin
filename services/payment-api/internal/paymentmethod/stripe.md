# Stripe互換決済API

`stripe.go`の`Stripe`実装が呼び出すHTTP APIの仕様です。

Minikubeでは、実際のStripeではなく`mock-stripe`を呼び出します。

```text
MOCK_STRIPE_URL=http://mock-stripe:8090
```

コード上では、`Stripe`の`baseURL`に以下のパスを連結して呼び出します。

## 1. PaymentIntentを作成する

### エンドポイント

```http
POST {BASE_URL}/v1/payment_intents
Content-Type: application/json
```

### リクエスト例

```json
{
  "amount": 1500,
  "currency": "JPY",
  "idempotency_key": "order-123-payment",
  "return_url": "http://localhost:3000/purchases",
  "payment_method": "card"
}
```

各項目の意味：

| 項目 | 内容 |
|---|---|
| `amount` | 決済金額。0より大きい値 |
| `currency` | 外部決済では`JPY` |
| `idempotency_key` | 同じ決済を二重実行しないためのキー |
| `return_url` | 3DS認証完了後の戻り先 |
| `payment_method` | `card`、`credit_card`、`bank_transfer`など |

### 成功レスポンス

自動確定が有効な場合は、次のように返ります。

```json
{
  "id": "pi_mock_1",
  "status": "succeeded"
}
```

`MOCK_STRIPE_AUTO_CONFIRM=false`の場合、追加認証が必要なレスポンスになります。

```json
{
  "id": "pi_mock_1",
  "status": "requires_action",
  "action_url": "http://mock-stripe:8090/3ds/pi_mock_1"
}
```

`stripe.go`は`requires_action`または`succeeded`なら、その結果をそのままpayment-apiへ返します。

## 2. PaymentIntentを確定する

PaymentIntent作成時のステータスが`succeeded`または`requires_action`ではない場合に、`stripe.go`が呼び出します。

### エンドポイント

```http
POST {BASE_URL}/v1/payment_intents/{payment_intent_id}/capture
Content-Type: application/json
```

### リクエスト例

```http
POST http://mock-stripe:8090/v1/payment_intents/pi_mock_1/capture
Content-Type: application/json

{}
```

### レスポンス例

```json
{
  "id": "pi_mock_1",
  "status": "succeeded"
}
```

`status`が`succeeded`でなければ、payment-apiは予期しない決済状態としてエラーにします。

## 3. PaymentIntentをキャンセルする

capture失敗時や、payment-api側の最終更新に失敗した場合の補償処理で呼び出されます。

### エンドポイント

```http
POST {BASE_URL}/v1/payment_intents/{payment_intent_id}/cancel
Content-Type: application/json
```

### リクエスト例

```http
POST http://mock-stripe:8090/v1/payment_intents/pi_mock_1/cancel
Content-Type: application/json

{}
```

### レスポンス例

```json
{
  "id": "pi_mock_1",
  "status": "canceled"
}
```

## 4. 返金する

`RefundExternal`から呼び出されます。

### エンドポイント

```http
POST {BASE_URL}/v1/refunds
Content-Type: application/json
```

### リクエスト例

```json
{
  "payment_intent_id": "pi_mock_1",
  "amount": 1500,
  "idempotency_key": "refund-order-123"
}
```

### レスポンス例

```json
{
  "id": "re_refund-order-123",
  "status": "succeeded"
}
```

## 5. 3DS認証画面を表示する

`requires_action`のレスポンスに含まれる`action_url`へブラウザを遷移させます。

### 認証画面

```http
GET {BASE_URL}/3ds/{payment_intent_id}
```

このエンドポイントはJSONではなく、認証用HTMLを返します。

### 認証完了

認証画面のボタンから、次のエンドポイントが呼ばれます。

```http
POST {BASE_URL}/3ds/{payment_intent_id}/confirm
```

mock Stripeは認証完了後、payment-apiのWebhookへ次のイベントを送ります。

```json
{
  "id": "evt_pi_mock_1_payment_intent.succeeded",
  "type": "payment_intent.succeeded",
  "data": {
    "id": "pi_mock_1",
    "status": "succeeded"
  }
}
```

payment-apiはこのWebhookを受けて、決済を確定し、`payments.succeeded.v1`をoutboxへ保存します。

## エラーの扱い

`stripe.go`の`post`はHTTPステータスを次のように扱います。

| HTTPステータス | 扱い |
|---|---|
| 2xx | JSONレスポンスを読み込む |
| 400〜499 | `PermanentError`。入力や決済内容の恒久的な失敗 |
| 429 | 一時的なレート制限。再試行可能 |
| 5xx | 一時的な外部サービス障害として扱う |

HTTP通信が成功しても、レスポンスJSONをデコードできない場合はエラーになります。

