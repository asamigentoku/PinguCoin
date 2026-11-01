# bff

Next.jsから利用する公開REST APIです。認証はClerk、内部サービスとの通信はgRPCを使用します。BFFは業務データベースを持ちません。

主なエンドポイント:

- `GET/POST /api/v1/products`
- `GET/PATCH/DELETE /api/v1/products/{id}`
- `GET /api/v1/products/{id}/assets`
- `GET /api/v1/products/{id}/upload-targets`
- `POST /api/v1/orders`
- `GET /api/v1/orders/{id}`
- `GET /api/v1/points`

商品とアセットは`media-api`、注文と購入済み判定は`order-api`、ポイント参照は`payment-api`へ委譲します。署名付きURLの発行でも、所有者または購入者であることをBFFで確認します。

```powershell
go run ./services/bff/cmd/api
```

既定ポートは`8082`です。`/healthz`と`/readyz`を公開します。
