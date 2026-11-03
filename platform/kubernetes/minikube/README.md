# minikubeでバックエンドを手動起動する

Terraformの適用と、次の `.env` の設定が完了している前提です。

- `services/media-api/.env`
- `services/payment-api/.env`
- `services/bff/.env`

フロントエンドは起動しません。以下のコマンドは、すべてリポジトリのルートからPowerShellで実行します。

## 1. minikubeを起動する

Docker Desktopを起動してから、minikubeを既定の設定で起動します。

```powershell
minikube start
```

起動状態を確認します。

```powershell
minikube status
kubectl cluster-info
```

`host`、`kubelet`、`apiserver` が `Running` になっていれば次へ進みます。

## 2. namespaceを作成する

バックエンドを配置する `pingucoin` namespaceを作成します。

```powershell
kubectl apply -f platform/kubernetes/minikube/namespace.yaml
```

`namespace/pingucoin created` または `namespace/pingucoin unchanged` と表示されれば成功です。

## 3. `.env`をKubernetes Secretへ登録する

実在する3つの `.env` を、それぞれKubernetes Secretへ登録します。

```powershell
kubectl -n pingucoin create secret generic media-api-env --from-env-file=services/media-api/.env --dry-run=client -o yaml | kubectl apply -f -
kubectl -n pingucoin create secret generic payment-api-env --from-env-file=services/payment-api/.env --dry-run=client -o yaml | kubectl apply -f -
kubectl -n pingucoin create secret generic bff-env --from-env-file=services/bff/.env --dry-run=client -o yaml | kubectl apply -f -
```

`--dry-run=client -o yaml | kubectl apply -f -` を付けているため、Secretが未作成なら作成され、すでに存在すれば更新されます。

登録結果を確認します。値そのものは表示されません。

```powershell
kubectl get secrets -n pingucoin
```

次の3つが表示されることを確認します。

- `media-api-env`
- `payment-api-env`
- `bff-env`

`inventory-api` と `order-api` はminikubeマニフェスト内のローカル設定を使うため、`.env` とSecretの登録は不要です。

## 4. バックエンドイメージをビルドする

Docker Desktop上で、BFFと4つのAPIを順番にビルドします。最後の `.` はリポジトリのルートをビルドコンテキストとして渡すために必要です。

```powershell
docker build -t pingucoin/media-api:dev -f services/media-api/Dockerfile .
docker build -t pingucoin/inventory-api:dev -f services/inventory-api/Dockerfile .
docker build -t pingucoin/payment-api:dev -f services/payment-api/Dockerfile .
docker build -t pingucoin/order-api:dev -f services/order-api/Dockerfile .
docker build -t pingucoin/bff:dev -f services/bff/Dockerfile .
```

Docker Desktopに5つのイメージが作成されたことを確認します。

```powershell
docker images --filter "reference=pingucoin/*:dev"
```

## 5. イメージをminikubeへ取り込む

`docker build` の出力に `docker:desktop-linux` と表示される場合、イメージはDocker Desktopにあり、minikubeからは直接参照できません。ビルドした5つのイメージをminikubeへ明示的にロードします。

```powershell
minikube image load pingucoin/media-api:dev
minikube image load pingucoin/inventory-api:dev
minikube image load pingucoin/payment-api:dev
minikube image load pingucoin/order-api:dev
minikube image load pingucoin/bff:dev
```

5つのイメージがminikube内に存在することを確認します。

```powershell
minikube image ls | Select-String -Pattern "pingucoin/"
```

次のイメージが表示されれば成功です。

- `pingucoin/media-api:dev`
- `pingucoin/inventory-api:dev`
- `pingucoin/payment-api:dev`
- `pingucoin/order-api:dev`
- `pingucoin/bff:dev`

## 6. Kubernetesマニフェストを適用する

PostgreSQL、Kafka、各API、BFF、監視コンポーネントをまとめて適用します。

```powershell
kubectl apply -k platform/kubernetes/minikube
```

マニフェストの適用後、Podの状態を確認します。

```powershell
kubectl get pods -n pingucoin
```

初回起動ではイメージの展開、PostgreSQLの初期化、各APIのマイグレーションに時間がかかることがあります。

## 7. バックエンドの起動完了を待つ

各Deploymentが利用可能になるまで待ちます。

```powershell
kubectl rollout status deployment/postgres -n pingucoin --timeout=5m
kubectl rollout status deployment/kafka -n pingucoin --timeout=5m
kubectl rollout status deployment/media-api -n pingucoin --timeout=5m
kubectl rollout status deployment/inventory-api -n pingucoin --timeout=5m
kubectl rollout status deployment/payment-api -n pingucoin --timeout=5m
kubectl rollout status deployment/order-api -n pingucoin --timeout=5m
kubectl rollout status deployment/bff -n pingucoin --timeout=5m
```

すべてのコマンドで `successfully rolled out` と表示されれば、バックエンドの起動は完了です。

最終状態を確認します。

```powershell
kubectl get deployments -n pingucoin
kubectl get pods -n pingucoin
kubectl get services -n pingucoin
```

## 8. BFFをローカルへ公開する

新しいPowerShellを開き、BFFを `localhost:8082` へport-forwardします。

```powershell
kubectl -n pingucoin port-forward service/bff 8082:8082
```

このコマンドは起動したままにします。停止するときは `Ctrl+C` を押します。

BFFには次のURLでアクセスできます。

```text
http://localhost:8082
```

## 起動に失敗した場合

まずPodの状態とイベントを確認します。

```powershell
kubectl get pods -n pingucoin
kubectl get events -n pingucoin --sort-by=.metadata.creationTimestamp
```

特定サービスのログを確認する場合は、Deployment名を指定します。

```powershell
kubectl logs -n pingucoin deployment/media-api
kubectl logs -n pingucoin deployment/inventory-api
kubectl logs -n pingucoin deployment/payment-api
kubectl logs -n pingucoin deployment/order-api
kubectl logs -n pingucoin deployment/bff
```

Podが `Pending`、`CrashLoopBackOff`、`ImagePullBackOff` になった場合は、対象Podの詳細を確認します。

```powershell
kubectl describe pod -n pingucoin <pod-name>
```

`ErrImageNeverPull` の場合は、対象のバックエンドイメージがminikube内にありません。手順5の `minikube image load` とイメージ一覧の確認をやり直します。

コードを変更してイメージを再ビルドした場合は、対象Deploymentを再起動します。

```powershell
kubectl rollout restart deployment/<deployment-name> -n pingucoin
kubectl rollout status deployment/<deployment-name> -n pingucoin --timeout=5m
```
