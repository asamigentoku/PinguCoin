# ログの形式

3つのサービス(orcan-api / payment-api / pingu-api)のログは、**同じ形式**にそろえています。Datadog の標準属性に合わせた、1行が1つの JSON です(標準出力)。実装は `pkg/logging` です。

## なぜそろえるか

サービスごとに、同じ意味の属性が、別の名前(`clientIP` / `client_ip_address` / `remote_address` ...)になっていると、サービスをまたいで検索・集計できません。名前をそろえておくと、ログ管理(Datadog)で、次のことができます。

- 全サービスの、`http.status_code` が 500 のログを、1つの検索で見る。
- **`request_id` で検索して、1つのリクエストが、どのサービスを通り、どこで失敗したかを追う**(下の「リクエスト ID」)。
- `service` / `env` / `version` で絞る(「production の v1.2.3 のログ」)。

## 属性

### 予約属性(すべてのログに付く)

| 属性 | 内容 |
| --- | --- |
| `timestamp` | 時刻(RFC 3339、UTC。末尾が `Z`) |
| `status` | レベル(`debug` / `info` / `warn` / `error`) |
| `message` | ログの本文 |
| `service` | サービス名(`orcan-api` / `payment-api` / `pingu-api`) |
| `env` | 環境(`local` / `minikube` / `staging` / `production`)。環境変数 `DD_ENV`(無ければ `APP_ENV`)。Kubernetes では、ConfigMap で全サービスにそろえて渡す |
| `version` | サービスのバージョン(リリースのタグ。`pkg/version`。詳しくは [VERSIONING.md](VERSIONING.md)) |

### 標準属性(該当するログに付く)

JSON の入れ子は、Datadog では `http.method` のように、ドットでつながって見えます。

| 属性 | 内容 | 出す所 |
| --- | --- | --- |
| `http.method` / `http.url` / `http.status_code` | HTTP のメソッド、パス、ステータス | pingu-api のリクエストログ |
| `http.useragent` / `http.referer` / `http.version` | User-Agent、Referer、HTTP のバージョン | 同上 |
| `network.client.ip` | クライアントの IP(Ingress の後ろでは `X-Forwarded-For` の先頭) | 同上 |
| `rpc.system` / `rpc.service` / `rpc.method` / `rpc.grpc.status_code` | gRPC の呼び出し(OpenTelemetry の命名) | orcan-api / payment-api のリクエストログ |
| `duration` | 処理時間(**ナノ秒の整数**。Datadog の duration の単位) | リクエストログ、DB のクエリ |
| `error.kind` / `error.message` | エラーの種類と本文 | エラーのあるログ |
| `db.statement` / `db.rows_affected` | DB のクエリと、影響した行数 | GORM のログ(遅いクエリ・失敗) |
| `request_id` | リクエスト ID | 下を参照 |

### ログに出さないもの

- 秘密(パスワード、トークン、接続文字列、Clerk のキー)。
- リクエストの本文(GraphQL のクエリや変数、注文の内容)。
- クエリ文字列(`?token=...` のような秘密を含みうる)。`http.url` には、**パスだけ**を出します。

## 例

pingu-api の HTTP リクエストのログ:

```json
{"timestamp":"2026-10-01T14:42:05.717Z","status":"info","message":"http request completed","service":"pingu-api","env":"production","version":"v1.2.3","http":{"method":"POST","url":"/api/v1/orders","status_code":201,"useragent":"Mozilla/5.0","referer":"","version":"HTTP/1.1"},"network":{"client":{"ip":"203.0.113.7"}},"duration":4217000,"request_id":"7f3c2b1a9e8d4c6ba5f4123456789abc"}
```

orcan-api の gRPC の呼び出しのログ(在庫不足で、リクエストが拒否された):

```json
{"timestamp":"2026-10-01T14:42:05.712Z","status":"warn","message":"grpc request rejected","service":"orcan-api","env":"production","version":"v1.2.3","rpc":{"system":"grpc","service":"orcan.v1.ProductInventoryService","method":"AdjustProductInventory","grpc":{"status_code":"FailedPrecondition"}},"duration":812000,"request_id":"7f3c2b1a9e8d4c6ba5f4123456789abc","error":{"kind":"FailedPrecondition","message":"insufficient stock"}}
```

この2つは、同じ `request_id` です。pingu-api の「注文」のリクエストが、orcan-api の在庫の消費で失敗した、という流れが、`request_id` の検索で分かります。

## リクエスト ID

```text
ブラウザ ─ X-Request-Id ─▶ pingu-api ─ gRPC メタデータ x-request-id ─▶ orcan-api / payment-api
```

- pingu-api は、リクエストの `X-Request-Id` ヘッダーを使います。無い、または安全でない(使える文字は英数字と `.` `_` `-` だけ、128 文字まで)ときは、新しく作ります。ログの偽装や、改行の混入を防ぐためです。
- レスポンスにも、同じ `X-Request-Id` を付けます(問い合わせのときに、利用者から ID を聞けます)。
- pingu-api が orcan-api / payment-api を gRPC で呼ぶとき、同じ ID を `x-request-id` で渡します。**再試行のときも、同じ ID** です([RETRY.md](RETRY.md))。
- orcan-api / payment-api も、その ID を、すべてのリクエストログに出します。

## レベル

- `info`: 正常。
- `warn`: クライアント起因の失敗(HTTP 4xx、gRPC の `InvalidArgument` / `NotFound` / `FailedPrecondition` など)、遅い DB クエリ。
- `error`: サーバー起因の失敗(HTTP 5xx、gRPC の `Internal` など)。**内部のエラーの本当の原因は、ここにだけ出ます**(クライアントには、汎用のメッセージだけを返します)。
- ヘルスチェック(`/healthz`、`/readyz`、gRPC の `Health`)は、数秒おきに呼ばれるので、ログに出しません。
- 出すレベルの下限は、環境変数 `LOG_LEVEL`(`debug` / `info` / `warn` / `error`。既定は `info`)。

## 使い方(コード)

```go
logger := logging.NewFromEnv("orcan-api") // service / env / version が、すべてのログに付く

logger.Error("failed to connect database", logging.Err(err))                          // error.kind / error.message
logger.InfoContext(ctx, "order created", logging.RequestID(ctx), logging.Duration(d)) // request_id / duration(ns)
logger.Info("applied", slog.Group("db", slog.String("statement", sql)))               // db.statement
```

属性の名前は、`pkg/logging` のコメントの表にそろえてください。新しい種類のログを足すときも、同じ意味の属性は、同じ名前にします。
