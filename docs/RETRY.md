# リトライの方針

通信が一時的に失敗したときの、再試行(リトライ)の方針です。

> **失敗したら何でも再試行する、のではありません。** 再試行は、「一時的な失敗」で、かつ「同じ操作を2回やっても結果が変わらない(冪等な)操作」のときだけです。待ち時間は、指数バックオフ(少しずつ伸ばす)とジッター(ランダムなばらつき)で、回数にも上限があります。

## 3つの原則

| 原則 | 内容 |
| --- | --- |
| 1. 一時的な失敗だけ | つながらない、相手が起動中、502 / 503 は再試行する。入力の間違い、未ログイン、在庫不足、ポイント不足などは、何度やっても同じなので、再試行しない。**タイムアウトも再試行しない**(相手が遅いときに、さらに負荷をかけて悪化させないため) |
| 2. 冪等な操作だけ | 読み取りと、冪等性キー(`Idempotency-Key`)が付いた書き込みだけ。商品の作成・更新、ポイントの付与・消費、返金などは、1回目が実は成功していたら二重になるので、**再試行しない** |
| 3. 上限がある | 回数(最大3回)と待ち時間(最大1〜2秒)に上限がある。障害で失敗が続いているときは、再試行を控える(再試行の集中を防ぐ) |

## どこで何をしているか

| 通信 | 再試行 | 方式・設定 | 場所 |
| --- | --- | --- | --- |
| pingu-api → orcan-api / payment-api (gRPC) | する(冪等なメソッドだけ) | gRPC の service config。最大 3 回、100ms → 200ms(上限 1 秒)、`UNAVAILABLE` だけ、`retryThrottling` あり | `services/pingu-api/internal/grpcclient/resilience.go`、メソッドの一覧は `orcanclient/retryable.go` と `paymentclient/retryable.go` |
| 同上の期限 | 1回の呼び出し(再試行を含む)に、10 秒の期限 | インターセプター。呼び出し元が期限を付けていれば、それを使う | 同上 |
| 起動時の DB への接続 | する(一時的な失敗だけ) | 最大 6 回、500ms → 5 秒(約 15 秒)。接続文字列の間違い・パスワードの違い・DB がない場合は、すぐに終了 | `pkg/dbretry`、`pkg/retry` |
| Next.js → pingu-api(GraphQL の query、`GET /points`、`GET /orders`) | する | 最大 3 回、200ms → 2 秒、つながらない・502・503 だけ | `apps/client-web/src/lib/retry.ts`、`api.ts`、`shop.ts`、`seller.ts` |
| Next.js → pingu-api(`POST /orders`) | する(**同じ `Idempotency-Key` で**) | 同上。在庫・決済・注文のすべてが、同じキーの再送を二重に処理しない | `apps/client-web/src/lib/shop.ts` |
| ブラウザ → Azure Blob(アップロード) | する | 最大 3 回、接続失敗と、500 / 502 / 503 だけ。同じ名前への上書きなので、結果は同じ。403(署名付き URL の期限切れ)などは、すぐにエラー | `apps/client-web/src/components/sell/listing-form.tsx` |
| orcan-api → Azure Blob | する(Azure SDK の標準) | SDK の標準のリトライ(指数バックオフ) | `azblob` の既定 |
| Next.js → pingu-api(商品の作成・更新・削除、ファイルの確定など) | **しない** | 1回目が成功していた場合に、二重に作られるため | — |

### gRPC で、再試行してよいメソッド

- 読み取り: `Get*` / `List*`、ダウンロード用 URL の発行
- 冪等性キーのある書き込み: `AdjustProductInventory`(在庫の消費)、`CreatePayment`(決済)
- find-or-create: `EnsureUser`(Clerk の ID で、あれば返し、なければ作る)

**再試行しない**もの: `Create*` / `Update*` / `Delete*` / `Confirm*`(商品・カテゴリー・アセットなど)、`CreditPoints` / `DebitPoints`(ポイントの付与・消費)、`RefundPayment` / `CancelPayment`(返金・キャンセル)。

メソッド名は、生成コードの定数(`..._FullMethodName`)で書いているので、書き間違いはビルドエラーになります。さらに、`retryable_test.go` が、「2回やると結果が変わる書き込み」が、一覧にうっかり入らないことを確かめます。

## エラーの返し方

pingu-api は、上流(orcan-api / payment-api)につながらなかったときに、HTTP **503**(`service temporarily unavailable`)を返します(再試行しても、つながらなかったあと)。上流が時間切れだったときは **504** です。上流のメッセージ(ホスト名など)は返しません。

- フロントエンドは、503 を、一時的な失敗として、冪等な操作なら再試行します。
- 504 は、再試行しません(タイムアウトを再試行しない原則)。

## 変えたいとき

- **回数・待ち時間(gRPC)**: `grpcclient/resilience.go` の `MaxAttempts` と、`ServiceConfig` の `initialBackoff` など。
- **期限(gRPC)**: `grpcclient.DefaultTimeout`(既定 10 秒)。pingu-api の HTTP の `WriteTimeout`(30 秒)は、これより長くしておくこと。
- **再試行するメソッドを増やす**: 冪等であることを確かめてから、`retryable.go` の一覧に足す(足していいかは、テストが確かめる)。
- **DB の起動時の待ち**: `pkg/retry` の `Startup`。
- **フロントエンド**: `retry.ts` の `withRetry` のオプション(`attempts` / `baseMs` / `maxMs`)。

## 関連する設定

リトライと合わせて、次も入れています。

- pingu-api の HTTP サーバーに、タイムアウト(`ReadHeaderTimeout` 5 秒、`ReadTimeout` 15 秒、`WriteTimeout` 30 秒、`IdleTimeout` 60 秒)と、SIGTERM を受けたときの安全な停止(処理中のリクエストを、最大 25 秒待つ)。
- Kubernetes の probe と再起動は [platform/kubernetes/production/servers.yaml](../platform/kubernetes/production/servers.yaml)(クラッシュしたときの再起動は、Kubernetes が、バックオフつきで行う)。

## テスト

| 対象 | テスト |
| --- | --- |
| 共通のリトライ部品(Go) | `pkg/retry/retry_test.go`: 回数の上限、バックオフの伸び方、ジッターの範囲、再試行しないエラー、context の中断 |
| DB の接続 | `pkg/dbretry/dbretry_test.go`: 一時的な失敗は再試行、設定の間違いはすぐに失敗 |
| gRPC | `grpcclient/resilience_test.go`: 実際の gRPC のやり取りで、読み取りは再試行する、入力の間違い・在庫不足・タイムアウトは再試行しない、書き込みは再試行しない、冪等な書き込みは再試行する、再試行のたびにトークンが付く、期限 |
| gRPC のメソッド一覧 | `orcanclient/retryable_test.go`、`paymentclient/retryable_test.go`: 危険な書き込みが入らない |
| フロントエンド | `apps/client-web/src/lib/retry.test.ts`(`npm test`) |
