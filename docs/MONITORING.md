# 監視(Prometheus + Grafana)

3つのサービス(orcan-api / payment-api / pingu-api)の状態を、**数(メトリクス)**で見えるようにしています。ローカル(minikube)と本番(AKS)で、**同じ構成**です。

| | 役目 |
| --- | --- |
| 各 API の `/metrics` | 自分の状態を、数で出す(リクエスト数、エラー、処理時間、DB の接続、注文の数、…) |
| **Prometheus** | 各 Pod の `/metrics` を、定期的に取りに行って(scrape)、保存する。アラートの条件も、ここで評価する |
| **Grafana** | Prometheus のデータを、グラフにして見る(ダッシュボード 7 枚) |
| kube-state-metrics | Kubernetes の「状態」(Pod の再起動・Ready・レプリカ数・リソースの上限)を、数にする |
| cAdvisor(kubelet に付属) | コンテナの CPU・メモリ・ネットワーク |

ログ([LOGGING.md](LOGGING.md))は「1件ずつの出来事」、メトリクスは「数の推移」です。両方を、`request_id` と、`service` / `version` で、行き来できます。

```text
                    ┌──────────── Kubernetes(namespace: pingucoin)────────────┐
 ブラウザ ─HTTPS─▶ Ingress ─▶ pingu-api :8082 ─gRPC─▶ orcan-api / payment-api
                    │              │ :9090 /metrics        │ :9090 /metrics   │
                    │              ▼                       ▼                  │
                    │         ┌──────────── Prometheus ───────────┐            │
                    │         │  15〜30 秒おきに scrape / 保存     │◀ kube-state-metrics, cAdvisor
                    │         │  アラートのルールを評価           │            │
                    │         └───────────────┬────────────────────┘            │
                    │                         ▼                                │
                    │                      Grafana ◀── 管理者が port-forward で開く
                    └──────────────────────────────────────────────────────────┘
```

## 見かた

```bash
bash script/monitor.sh        # Grafana → http://localhost:3001  、Prometheus → http://localhost:9090
```

- minikube では、`bash script/start.sh` のあとに、そのまま使えます。ログイン不要(ローカルだけの設定)。
- 本番では、`az aks get-credentials` した、AKS の管理者が、同じコマンドで開きます。ログインは、ユーザー `admin` と、Key Vault の `grafana-admin-password`(Terraform が作った値)です。
  ```bash
  az keyvault secret show --vault-name <key-vault-name> --name grafana-admin-password --query value -o tsv
  ```
- Grafana のポートが 3001 なのは、フロントエンド(`npm run dev`)が 3000 を使うためです。
- **Grafana も Prometheus も、インターネットには公開していません**(Ingress が無い)。`/metrics` も同じです(下の「安全性」)。

## ダッシュボード(フォルダ: PinguCoin)

| ダッシュボード | 見るもの | こんなときに |
| --- | --- | --- |
| **概要** | 動いている API・発火中のアラート・リクエスト数・5xx の割合・p95・業務(注文・決済)・CPU・メモリ | まず、ここ。「いま、おかしくないか」 |
| **HTTP と GraphQL** | ルート別のリクエスト数・ステータス・応答時間・処理中の数、GraphQL の操作別の数と失敗、認証の結果 | 「どの API が、遅い・失敗している」 |
| **gRPC** | サーバー側(orcan-api / payment-api)と、クライアント側(pingu-api から)の、数・結果のコード・時間・**再試行** | 「サービス間の呼び出しが、おかしい」 |
| **データベース** | 接続プール(使用中・待ち)、クエリの数・失敗・処理時間(テーブル・操作別) | 「DB が、遅い・つながらない・接続が足りない」 |
| **業務** | 注文(結果別)・売上・決済・返金・ポイント・在庫・出品・Blob | 「システムは動いているが、売れていない・決済だけ失敗している」 |
| **Kubernetes** | Pod の状態・再起動・OOMKilled・CPU・メモリ(requests / limits)・スロットリング・ネットワーク・ノード・Prometheus 自身 | 「Pod が落ちる・リソースが足りない」 |
| **Go ランタイム** | goroutine・ヒープ・GC・プロセスのメモリ・開いているファイルの数・稼働時間・**動いているバージョン** | 「メモリリーク・goroutine のリークの疑い」「デプロイで、切り替わったか」 |

ダッシュボードは、ファイルで管理しています(`platform/kubernetes/*/monitoring/dashboards/*.json`)。**画面での編集は、保存できません**(コードと食い違わないため)。直したいときは、[ダッシュボードを直す](#ダッシュボードを直す)。

## アラート

ルールは `platform/kubernetes/*/monitoring/alert-rules.yml`。条件が、一定時間(`for`)続くと、**firing(発火)**になります。

| 区分 | アラート | 重大度 | 意味 |
| --- | --- | --- | --- |
| 稼働 | `ServiceDown` | critical | API の `/metrics` が、2 分以上取れない |
| | `PodNotReady` | warning | Pod が、5 分以上 Ready でない(DB に接続できない) |
| | `PodRestarting` / `ContainerOOMKilled` | warning / critical | 再起動した / メモリ上限で、強制終了された |
| | `DeploymentReplicasUnavailable` | critical | 指定した数だけ、動いていない |
| エラー・遅さ | `HttpServerErrors` | critical | HTTP 5xx が 5% 超 |
| | `HttpSlowRequests` | warning | ルートの p95 が 1 秒超 |
| | `GrpcServerErrors` / `GrpcClientErrors` | critical | gRPC のサーバー側の失敗が、5% 超(入力の誤りは、含まない) |
| | `GrpcClientRetriesHigh` | warning | 再試行が 20% 超(相手が、不安定になりかけている) |
| | `GraphqlErrors` | warning | GraphQL の操作の 20% 超が、エラー(HTTP 200 のまま、失敗している) |
| DB | `DbConnectionWaits` / `DbConnectionPoolNearLimit` | warning | 接続が足りない |
| | `DbQueryErrors` / `DbSlowQueries` | critical / warning | クエリの失敗 / p95 が 0.5 秒超 |
| 業務 | `OrderFailures` / `PaymentFailures` / `BlobStorageFailures` | warning / critical / warning | **サーバー側の失敗**で、注文・決済・Blob の操作ができていない(在庫不足・ポイント不足は、含まない) |
| リソース | `ContainerMemoryNearLimit` / `ContainerCpuThrottled` | warning | メモリが上限の 90% 超 / CPU が上限で絞られている |

**通知(メール・Slack・電話)は、置いていません。** 発火は、次の場所で見えます。

- Grafana「概要」の「発火中のアラート」の表(と、「発火中のアラート」の数)
- Prometheus の `http://localhost:9090/alerts`

通知が必要になったら、Prometheus に **Alertmanager** を足して、`prometheus.yml` の `alerting:` に向けます(Slack・メールなどの宛先は、Alertmanager の設定)。コストを抑える間は、見に行く形で足ります。

アラートのルールは、**テスト**してあります(`alert-rules.test.yml`。`promtool test rules`。CI でも実行)。作ったメトリクスの値で、「発火する」だけでなく、**「発火しない」**を確かめます(在庫不足・入力の誤りでは出ない、トラフィックが少なすぎるときは出ない、など)。ルールを変えたら、テストも直してください。

```bash
docker run --rm --entrypoint promtool -v "$PWD/platform/kubernetes/minikube/monitoring:/m:ro" prom/prometheus:v3.5.0 test rules /m/alert-rules.test.yml
```

しきい値は、トラフィックが少ない前提です。割合のルールには、「一定以上のリクエストがあるとき」の条件を付けて、1 件の失敗で、割合が跳ねて発火しないようにしています。

## どんなメトリクスを出しているか

すべて `pingucoin_` で始まります(Go のランタイム `go_*` とプロセス `process_*` は、そのまま)。実装は `pkg/metrics`。

| 名前 | 種類 | ラベル | 何の数 |
| --- | --- | --- | --- |
| `pingucoin_build_info` | gauge | service, version, commit, go_version | バージョン(値は常に 1) |
| `pingucoin_http_requests_total` | counter | method, route, code | HTTP リクエスト(pingu-api)。route は、`/api/v1/orders/{id}` のような**パターン** |
| `pingucoin_http_request_duration_seconds` | histogram | method, route | 処理時間(認証など、API の手前も含む) |
| `pingucoin_http_in_flight_requests` | gauge | | 処理中の数 |
| `pingucoin_http_response_bytes_total` | counter | route | レスポンスの量 |
| `pingucoin_graphql_operations_total` | counter | type, field, result | GraphQL の操作(field = 最上位のフィールド名。result = ok / error) |
| `pingucoin_graphql_operation_duration_seconds` | histogram | type | GraphQL の処理時間 |
| `pingucoin_auth_requests_total` | counter | result | 認証の結果(anonymous / authenticated / invalid_token / profile_error) |
| `pingucoin_grpc_server_handled_total` | counter | grpc_service, grpc_method, grpc_code | gRPC サーバーが処理した数(ヘルスチェックは、除く) |
| `pingucoin_grpc_server_handling_seconds` | histogram | grpc_service, grpc_method | 処理時間 |
| `pingucoin_grpc_server_in_flight_requests` | gauge | | 処理中の数 |
| `pingucoin_grpc_client_handled_total` | counter | target, grpc_service, grpc_method, grpc_code | pingu-api からの呼び出し(再試行を含めた最終の結果) |
| `pingucoin_grpc_client_handling_seconds` | histogram | target, grpc_service, grpc_method | かかった時間(再試行の待ちを含む) |
| `pingucoin_grpc_client_attempts_total` | counter | target, grpc_service, grpc_method | **実際に送った回数**(再試行を含む)。`handled` との差が、再試行 |
| `pingucoin_db_connections_{open,in_use,idle,max_open}` | gauge | | 接続プールの状態 |
| `pingucoin_db_connection_wait_total` / `_wait_seconds_total` | counter | | 接続を待たされた回数・時間 |
| `pingucoin_db_connections_closed_*_total` | counter | | 閉じた接続(寿命・アイドル) |
| `pingucoin_db_queries_total` | counter | operation, table, result | クエリ(result = ok / not_found / error) |
| `pingucoin_db_query_duration_seconds` | histogram | operation, table | クエリの処理時間 |
| `pingucoin_orders_total` / `pingucoin_order_amount_points_total` | counter | result | 注文の結果 / 成立した注文の合計ポイント |
| `pingucoin_payments_total` / `pingucoin_payment_amount_total` | counter | method, result / currency | 決済の結果 / 成立した決済の合計額 |
| `pingucoin_refunds_total` | counter | result | 返金 |
| `pingucoin_point_transactions_total` / `pingucoin_point_amount_total` | counter | type, result / type | ポイントの付与(credit)・消費(debit) |
| `pingucoin_inventory_adjustments_total` | counter | direction, result | 在庫の増減 |
| `pingucoin_products_created_total` | counter | | 出品 |
| `pingucoin_blob_operations_total` / `_operation_duration_seconds` | counter / histogram | operation, result / operation | Azure Blob の操作 |

Prometheus が付ける `app`(サービス名)・`pod`・`node` のラベルが、どのメトリクスにも付きます。

### 結果(result)の意味

業務のメトリクスは、「**呼び出し側の問題**」と「**サーバーの失敗**」を、分けて数えます(アラートの対象にするのは、後者だけ)。

| result | 意味 | アラート |
| --- | --- | --- |
| `succeeded` / `created` | 成功 | |
| `replayed` | 同じ冪等性キーの再送。最初の結果を返した(二重処理は、していない) | |
| `rejected` | 入力の誤り・存在しない・未ログインなど | |
| `insufficient` / `out_of_stock` / `insufficient_point` | 在庫・ポイントが足りない(通常の結果) | |
| `failed` | **サーバーの失敗**(DB・ネットワーク・相手のサービス) | **対象** |

## 安全性(メトリクスを、どう守っているか)

- **別のポートで、別のサーバー。** `/metrics` は、API とは別のポート(`9090`。環境変数 `METRICS_PORT`)で、別の HTTP サーバーが出します。インターネットに向けた入口(Ingress → pingu-api の `8082`)からは、**そもそも届きません**。API のルーターにも、`/metrics` は無い(テストで確かめています)。
- **Prometheus からだけ届く。** NetworkPolicy が、`9090` への入ってくる通信を、Prometheus の Pod だけに許可します(`monitoring/network-policy.yaml`)。同じ namespace の、ほかの Pod からも、見えません。
- **Prometheus・Grafana も、非公開。** Ingress も、外向きの Service も、ありません。見るには、クラスターの権限(`kubectl`)が要ります。本番の Grafana は、さらに、パスワードが要ります(匿名アクセスと、利用者の登録は、無効)。
- **秘密を出さない。** メトリクスには、パスワード・トークン・リクエストの本文・クエリ文字列・SQL の値を、出しません(ラベルにも、入れません)。
- **ラベルを増やさない(カーディナリティ)。** ラベルの値は、有限の固定の集合だけ。次のものは、**入れません**。入れると、時系列が、際限なく増えて、Prometheus のメモリとディスクを、食いつぶします(利用者が、意図せず、または悪意で、増やせてしまう)。
  - ユーザー ID・商品 ID・リクエスト ID・生の URL のパス(→ route は、パターン。合わない URL は、`unmatched` の 1 つ)
  - クライアントが決められる文字列(GraphQL の操作の名前 → 最上位のフィールド名。支払い方法 → 既知の値以外は `other`)
- **監視が、本番を止めない。** メトリクスのサーバーが失敗しても、API は止めません(ログに出すだけ)。デプロイでも、監視の起動に失敗して、警告が出るだけで、デプロイは、止めません。

## 構成と、リソース

| | ローカル(minikube) | 本番(AKS) |
| --- | --- | --- |
| 場所 | `platform/kubernetes/minikube/monitoring/` | `platform/kubernetes/production/monitoring/` |
| scrape の間隔 | 15 秒 | 30 秒 |
| Prometheus の保存先 | emptyDir(Pod を作り直すと、消える) | **PVC 5 GiB**(Azure Disk、Pod を作り直しても残る)。7 日分、または 4 GB まで |
| Grafana のログイン | 不要(ローカル専用) | admin + Key Vault のパスワード |
| メモリの要求 / 上限(合計) | 約 370 Mi / 約 1 GiB | 約 430 Mi / 約 1 GiB |
| (実測、ローカル) | Prometheus 約 45 Mi、Grafana 約 200 Mi、kube-state-metrics 約 15 Mi | |

2つの環境は、**共有していません**(Kubernetes を共通化しない方針)。片方を直したら、もう片方も、同じように直してください(ダッシュボードの JSON は、同じものです)。

本番は、**ノードを増やしません**(既存の `Standard_B2ms` に載ります)。増える費用は、ディスク(5 GiB)の、月 1 ドル未満です([COST.md](COST.md))。

## 使い方

### 変更の確かめかた(PromQL の例)

Prometheus の画面(`http://localhost:9090`)の「Graph」で、次のように試せます。

```promql
# pingu-api の、5xx の割合(5 分)
sum(rate(pingucoin_http_requests_total{code=~"5.."}[5m])) / sum(rate(pingucoin_http_requests_total[5m]))

# ルート別の p95
histogram_quantile(0.95, sum by (le, route) (rate(pingucoin_http_request_duration_seconds_bucket[5m])))

# orcan-api への呼び出しの、再試行の割合
1 - sum(rate(pingucoin_grpc_client_handled_total{target="orcan-api"}[5m])) / sum(rate(pingucoin_grpc_client_attempts_total{target="orcan-api"}[5m]))

# 直近 1 時間の売上(ポイント)
sum(increase(pingucoin_order_amount_points_total[1h]))
```

「Status > Targets」で、取得の対象(`pingucoin-apps` が、3 つの API の Pod の数だけ UP か)を確かめられます。

### 障害のとき

1. 「概要」で、**発火中のアラート**と、5xx・p95 を見る。
2. 「HTTP と GraphQL」で、どのルートか。「gRPC」で、どの呼び出しか(**再試行**が増えていないか)。「データベース」で、接続待ち・遅いクエリ。
3. 「Kubernetes」で、再起動・OOMKilled・CPU の絞り・メモリの上限。
4. 時刻を絞って、ログへ(Datadog 形式。`request_id` で、サービスをまたいで追える。[LOGGING.md](LOGGING.md))。

### 新しいメトリクスを足す

1. 数える場所に、`pkg/metrics` の関数(または、`promauto` のカウンター)を足す。名前は `pingucoin_<何>_<単位>`、カウンターは `_total`、時間は `_seconds`。
2. **ラベルの値が、有限か**を、必ず確かめる(上の「ラベルを増やさない」)。ユーザー入力は、ラベルに入れず、`metrics.Label(値, 許可する値...)` で、既知の値だけにする。
3. テストを足す(`pkg/metrics/metrics_test.go` を参考に。カウンターの「増えた分」を比べる)。
4. ダッシュボードに、パネルを足す(下)。

### ダッシュボードを直す

JSON を手で書くのは、長くて、間違えやすいので、**Grafana の画面で作って、JSON を書き出す**のが楽です。

1. ローカルの Grafana で、ダッシュボードを開いて、パネルを足す(画面では、保存できないので、「Save as」ではなく、**Export > Export as JSON**)。
2. `platform/kubernetes/minikube/monitoring/dashboards/` の、対応する JSON を置き換える。**本番側にも、同じ JSON を置く**。
3. `bash script/start.sh --skip-build` で、反映する(ConfigMap の名前に、中身のハッシュが付くので、Grafana が、自動で作り直される)。

### アラートを足す・変える

`alert-rules.yml`(ローカルと、本番の両方)を直して、適用します。条件の式は、Prometheus の画面で、先に試してください。`for` で、一時的な揺れでは、発火しないようにします。

## 制限と、これから

- **通知は無い**(見に行く形)。Alertmanager を足すと、Slack・メールに送れる。
- **トレーシング**(1 つのリクエストが、サービスのどこで、何ミリ秒かかったか)は、無い。`request_id` で、ログから追う。必要なら、OpenTelemetry(Tempo など)を足す。
- **PostgreSQL 自体**(CPU・ストレージ・接続数)は、見ていない。アプリから見た、接続プールとクエリだけ。Azure のポータル(メトリクス)で見る。
- **ノードが 1 つ**の間は、ノードが落ちると、Prometheus と Grafana も、いっしょに止まる(ディスクの保存は、残る)。
- Prometheus の PVC は、**1 つの Pod だけ**が使う(`Recreate`)。更新のとき、数十秒、取得が途切れる。
- API の接続プールの上限(`MaxOpenConns`)は、設定していない(無制限)。3 つの API の接続の合計が、DB の上限を超えないかを、「データベース」の「開いている接続」で見る。
