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
| `start.sh` | 起動スクリプト(minikube 起動、イメージのビルド、Secret の作成、適用、待機) |
| `stop.sh` | 停止スクリプト(削除) |
| `namespace.yaml` | 専用の namespace `pingucoin` |
| `config.yaml` | クラスター内の宛先(ConfigMap) |
| `servers.yaml` | 3つの API の Deployment(Pod の定義)と Service(クラスター内の宛先) |
| `kustomization.yaml` | 上の3つの YAML をまとめて適用するための一覧 |

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

- `--skip-build` ... イメージのビルドを省略します(コードを変えていないとき)。
- `--port-forward` ... 起動後に `pingu-api` を `localhost:8082` に公開します(Ctrl+C で終了)。

`start.sh` は次の処理をします。

1. `server/<api>/.env` が揃っているか確認する
2. minikube を起動する(すでに動いていれば何もしない)
3. 3つの API のイメージをビルドする
4. namespace を作り、`.env` から Secret を作成する(すでにあれば更新する)
5. `kubectl apply -k` で適用し、Deployment を再起動して Pod の準備完了を待つ

`.env` を編集したときも、もう一度 `start.sh --skip-build` を実行すれば反映されます。

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
