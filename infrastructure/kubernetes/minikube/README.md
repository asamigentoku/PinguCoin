# Minikube ローカル環境

ローカルの Kubernetes(minikube)で、3つの Go サーバー(orcan-api / payment-api / pingu-api)だけを動かします。

**DB と Azure Storage はクラスター内に立ち上げません。** 各 API の `.env` に、Terraform で作った staging の Supabase(PostgreSQL)と Azure Blob Storage の接続情報を書いて使います(`infrastructure/terraform/envs/staging`)。

- API 同士の通信は、クラスター内のプライベートなネットワークで平文の gRPC(h2c)を使います。
- ホストに公開するのは `pingu-api` だけです(port-forward)。

> 注意: staging の**本物のデータ**に接続します。書き込みや削除に気をつけてください。
> Blob の保存パスを分けたい場合は、`orcan-api/.env` の `APP_ENV` を `staging` 以外(例: `minikube`)にしてください。

## 環境変数の仕組み

サーバーは `.env` ファイルを直接読まず、環境変数だけを読みます。minikube では次の流れで渡します。

```text
server/<api>/.env ──> Secret「<api>-env」──(envFrom)──> Pod の環境変数
                      config.yaml の ConfigMap ───────> 宛先アドレスを上書き
```

- `.env` は `.gitignore` に入っているので、git には入りません。
- `.env` の中身は、そのまま Secret(`orcan-api-env` / `payment-api-env` / `pingu-api-env`)になります。
- `.env` に `ORCAN_API_ADDR=localhost:8080` と書いてあっても、Pod 内では届きません。そのため `config.yaml` の ConfigMap が、`ORCAN_API_ADDR` と `PAYMENT_API_ADDR` をクラスター内の Service 名で上書きします。
- `PORT` は Service のポートと合わせるため、`servers.yaml` で固定しています(`.env` の値は無視されます)。

## ファイル構成

| ファイル | 内容 |
| --- | --- |
| `start.sh` | 起動スクリプト(minikube 起動、ホストの Docker でイメージをビルドして取り込み、Secret の作成、適用、待機) |
| `stop.sh` | 停止スクリプト(削除) |
| `namespace.yaml` | 専用の namespace `pingucoin` |
| `config.yaml` | クラスター内の宛先(ConfigMap) |
| `servers.yaml` | 3つの API の Deployment(Pod の定義)と Service(クラスター内の宛先)。リソース・ヘルスチェック・ローリングアップデートなどの設定も、ここに書いてあります |
| `autoscaling.yaml` | 自動スケール(HPA)と、同時に落とせる数の制限(PDB) |
| `network-policy.yaml` | 通信の制限(NetworkPolicy) |
| `kustomization.yaml` | 上の YAML をまとめて適用するための一覧 |

## 事前準備

- Docker
- Minikube
- kubectl
- bash(Windows は Git Bash、macOS / Linux は標準のシェル)。手動で起動する場合は PowerShell でも実行できます

## `.env` を用意する

各 API で `.env.example` をコピーして編集します(リポジトリのルートで実行)。

```bash
cp server/orcan-api/.env.example   server/orcan-api/.env
cp server/payment-api/.env.example server/payment-api/.env
cp server/pingu-api/.env.example   server/pingu-api/.env
```

staging に接続するために、最低限次を設定します。

| 変数 | 対象 | 内容 |
| --- | --- | --- |
| `DATABASE_URL` | 3つとも | Supabase の Connect 画面にある Transaction pooler URL。設定すると `DB_*` より優先されます |
| `AZURE_STORAGE_CONNECTION_STRING` | orcan-api | `terraform output -raw azure_storage_connection_string` の値 |
| `AZURE_STORAGE_PUBLIC_CONTAINER` / `AZURE_STORAGE_PRIVATE_CONTAINER` | orcan-api | `pingue-public` / `pingue` |
| `APP_ENV` | orcan-api | Blob パスの環境名 |
| `INTERNAL_API_TOKEN` | 3つとも | **同じ値**にします(pingu-api から内部 API を呼ぶときの共有トークン) |
| `CLERK_SECRET_KEY` | pingu-api | Clerk のシークレットキー |

## 起動(スクリプト)

リポジトリのどこからでも実行できます。

```bash
bash infrastructure/kubernetes/minikube/start.sh
```

オプション:

- `--skip-build` ... イメージのビルドを省略します(`.env` だけ変えたとき)。
- `--port-forward` ... 起動後に `pingu-api` を `localhost:8082` に公開します(Ctrl+C で終了)。
- `--only <api>` ... 指定した API だけビルドして再起動します(例: `--only pingu-api`。複数回指定できます)。

`start.sh` は次の処理をします。

1. `server/<api>/.env` が揃っているか、Docker が起動しているかを確認する
2. minikube を起動する(すでに動いていれば何もしない)
3. **ホストの Docker** で API のイメージをビルドし、`minikube image load` で minikube に取り込む
4. namespace を作り、`.env` から Secret を作成する(すでにあれば更新する)
5. `kubectl apply -k` で適用し、Deployment を再起動して Pod の準備完了を待つ

イメージのビルドに `minikube image build` を使わないのは、minikube 内でのビルドが遅く、メモリ不足で途中で落ちることがあるためです。ホストの Docker ならビルドのキャッシュも効きます。

`.env` を編集したときは、`start.sh --skip-build` を実行すれば反映されます。コードを変えたときは、変えた API だけ `--only` で指定すると早く終わります。

## 起動(手動)

スクリプトを使わずに、1つずつコマンドで起動する方法です。リポジトリのルートで実行します。コマンドは1行ずつ独立していて、bash と PowerShell のどちらでも動きます。

**1. minikube を起動する**

```bash
minikube start --driver=docker
```

**2. イメージをビルドする**(minikube 内の Docker に作られます)

```bash
minikube image build -t pingucoin/orcan-api:dev server/orcan-api
minikube image build -t pingucoin/payment-api:dev server/payment-api
minikube image build -t pingucoin/pingu-api:dev server/pingu-api
```

`-f` は付けません。`minikube image build` は `-f` をビルドコンテキスト(最後の引数のフォルダ)からの相対パスとして扱うので、`-f server/orcan-api/Dockerfile` と書くと Dockerfile が見つからずに失敗します。しかも、エラーを出さずに終了コード 0 で終わります。

イメージが入ったかは、次のコマンドで確認します。何も出ないときは、ビルドに失敗しています。

```bash
minikube image ls | grep pingucoin
```

**ビルドが終わらない・途中で落ちるとき**

minikube 内でのビルドは、メモリ不足で `go build` が途中で止まったり、`error reading from server: EOF` で落ちたりすることがあります。その場合は、ホストの Docker でビルドして、できたイメージを minikube に取り込みます。ホストのほうが CPU とメモリに余裕があるので、速く終わります。

```bash
docker build -t pingucoin/orcan-api:dev server/orcan-api
minikube image load pingucoin/orcan-api:dev
```

他の API も、名前を変えて同じように実行します。`docker build` だけではホスト側にイメージができるだけで、minikube からは見えません(`ErrImageNeverPull` の原因になります)。必ず `minikube image load` までやります。

**3. namespace を作る**

```bash
kubectl apply -f infrastructure/kubernetes/minikube/namespace.yaml
```

**4. `.env` から Secret を作る**

```bash
kubectl create secret generic orcan-api-env -n pingucoin --from-env-file=server/orcan-api/.env
kubectl create secret generic payment-api-env -n pingucoin --from-env-file=server/payment-api/.env
kubectl create secret generic pingu-api-env -n pingucoin --from-env-file=server/pingu-api/.env
```

すでに Secret がある場合は「AlreadyExists」で失敗します。`.env` を編集して更新したいときは、先に削除してから作り直します。

```bash
kubectl delete secret orcan-api-env payment-api-env pingu-api-env -n pingucoin
```

**5. マニフェストを適用して、起動を待つ**

```bash
kubectl apply -k infrastructure/kubernetes/minikube
kubectl wait --for=condition=ready pod --all -n pingucoin --timeout=180s
kubectl get pods -n pingucoin
```

**6. pingu-api を公開する**(プロセスは起動したままにします)

```bash
kubectl port-forward -n pingucoin service/pingu-api 8082:8082
```

**後から変更したとき**

`.env` を変更した場合は、手順4で Secret を作り直したあと、該当の Deployment を再起動します。

```bash
kubectl rollout restart -n pingucoin deployment/orcan-api
```

コードを変更した場合は、該当 API の手順2(ビルド)をもう一度実行してから、同じく `rollout restart` します。

## スケールと障害対策の設定

Kubernetes で一般的に設定するものを、3つの API すべてに入れています(設定の理由は YAML のコメントにも書いてあります)。

| 設定 | 場所 | 内容 |
| --- | --- | --- |
| `replicas: 2` | `servers.yaml` | 常に2つ動かし、1つ落ちてもサービスを止めない |
| ローリングアップデート | `servers.yaml` | `maxUnavailable: 0`。新しい Pod が Ready になってから古い Pod を止めるので、デプロイ中も止まらない |
| `resources` | `servers.yaml` | `requests`(確保する量。HPA の基準)と `limits`(上限。メモリを超えると OOMKilled で再起動) |
| ヘルスチェック(probe) | `servers.yaml` | 下の表を参照 |
| `preStop` の sleep と `terminationGracePeriodSeconds` | `servers.yaml` | 止める前に、Service の宛先から外れるのを 5 秒待つ。処理中のリクエストを落としにくくする |
| `topologySpreadConstraints` | `servers.yaml` | Pod を別のノードに分散させる(minikube は 1 ノードなので、置けなくてもエラーにしない) |
| `securityContext` | `servers.yaml` | root で動かさない、権限昇格を禁止、ルートファイルシステムを読み取り専用、capabilities を全部外す |
| `automountServiceAccountToken: false` | `servers.yaml` | Kubernetes API を呼ばないので、トークンを Pod に渡さない |
| HPA | `autoscaling.yaml` | CPU 使用率 70% を目安に 2〜5 個で自動スケール。増やすのは素早く、減らすのは 5 分落ち着いてから |
| PDB | `autoscaling.yaml` | ノードの drain などの間も、最低 1 つは動かし続ける |
| NetworkPolicy | `network-policy.yaml` | 入ってくる通信を既定で全部拒否し、orcan-api と payment-api は pingu-api からだけ受け付ける |

HPA は CPU 使用率を取るために metrics-server が必要です。`start.sh` が `minikube addons enable metrics-server` を実行します(有効になった直後は、`kubectl get hpa -n pingucoin` の `TARGETS` が数分間 `<unknown>` のままです)。

### ヘルスチェック

各 API にヘルスチェックを実装しています。

| API | 方式 | readiness(トラフィックを受けてよいか) | liveness(生きているか) |
| --- | --- | --- | --- |
| orcan-api / payment-api | gRPC 標準の `grpc.health.v1` | サービス名 `""`。DB に接続できる間だけ SERVING | サービス名 `liveness`。プロセスが動いている限り常に SERVING |
| pingu-api | HTTP | `GET /readyz`。DB に接続できる間だけ 200(できなければ 503) | `GET /healthz`。常に 200 |

- liveness に DB の状態を含めないのは、DB が一時的に落ちただけで Pod を再起動し続けないためです。DB が落ちたときは、readiness だけが落ちて Service の宛先から外れます(再起動はされません)。
- gRPC のヘルスチェックは、サービス間認証(`x-internal-token`)の対象外です。kubelet はトークンを持たないためです。返すのは「動いているか」だけです。
- どのヘルスチェックも、数秒おきに呼ばれてもログが埋まらないよう、リクエストログに出しません。
- orcan-api と payment-api は、SIGTERM を受けると readiness を落とし、処理中のリクエストが終わるのを待ってから止まります。

確認するコマンド:

```bash
kubectl get hpa,pdb,networkpolicy -n pingucoin        # 設定が入っているか
kubectl describe pod -n pingucoin -l app=pingu-api    # Events に probe の失敗が出ていないか
kubectl port-forward -n pingucoin service/pingu-api 8082:8082
curl localhost:8082/readyz                            # 別のターミナルで
```

## pingu-api にアクセスする

`http://localhost:8082/` で GraphQL Playground が開きます。GraphQL のエンドポイントは `http://localhost:8082/api/v1/graphql` です(port-forward を起動している間だけ使えます)。

gRPC サーバーを直接確認する場合:

```bash
kubectl port-forward -n pingucoin service/orcan-api 8080:8080
grpcurl -plaintext localhost:8080 list
```

アプリケーションの呼び出しには、`INTERNAL_API_TOKEN` の値をメタデータヘッダーに付けます。

```bash
grpcurl -plaintext -H "x-internal-token: <INTERNAL_API_TOKEN の値>" localhost:8080 orcan.v1.ProductService/ListProducts
```

## ログ

```bash
kubectl logs -n pingucoin deployment/pingu-api -f
kubectl logs -n pingucoin deployment/orcan-api -f
kubectl logs -n pingucoin deployment/payment-api -f
```

## 停止

スクリプト:

```bash
bash infrastructure/kubernetes/minikube/stop.sh                  # API と Secret を削除
bash infrastructure/kubernetes/minikube/stop.sh --stop-minikube  # minikube 自体も停止
```

手動:

```bash
kubectl delete -k infrastructure/kubernetes/minikube
minikube stop   # minikube 自体を止める場合
```

namespace `pingucoin` ごと削除するので、Secret も消えます。次回は起動のときに `.env` から作り直します。staging 側のデータは削除されません。
