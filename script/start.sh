#!/usr/bin/env bash
# minikube上にAPIを起動するスクリプト。
# イメージはホストのDockerでビルドし、minikubeに取り込む(`minikube image build` は使わない)。
# リポジトリのどこから実行しても動く。Git Bash / macOS / Linux 向け。
#
#   bash script/start.sh                  # ビルド + 取り込み + Secret作成 + 適用
#   bash script/start.sh --skip-build     # ビルドを省略(.env だけ変えたとき)
#   bash script/start.sh --port-forward   # 最後にpingu-apiを8082で公開
#   bash script/start.sh --only pingu-api # 指定したAPIだけビルド・再起動(複数指定可)
#
# 事前に services/<api>/.env を用意しておくこと(.env.example をコピーして編集)。
set -euo pipefail

NAMESPACE="pingucoin"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
K8S_DIR="$REPO_ROOT/platform/kubernetes/minikube"
ALL_APIS=(orcan-api payment-api pingu-api)
APIS=()

SKIP_BUILD=false
PORT_FORWARD=false
while [ $# -gt 0 ]; do
  case "$1" in
    --skip-build) SKIP_BUILD=true ;;
    --port-forward) PORT_FORWARD=true ;;
    --only)
      shift
      [ $# -gt 0 ] || { echo "--only needs an API name (${ALL_APIS[*]})" >&2; exit 1; }
      case " ${ALL_APIS[*]} " in *" $1 "*) APIS+=("$1") ;; *) echo "Unknown API: $1 (${ALL_APIS[*]})" >&2; exit 1 ;; esac
      ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done
[ ${#APIS[@]} -gt 0 ] || APIS=("${ALL_APIS[@]}")

# 1. 必要なコマンドと .env の確認
for tool in minikube kubectl docker; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is not installed or not in PATH." >&2; exit 1; }
done
docker info >/dev/null 2>&1 || { echo "Docker is not running. Start Docker Desktop first." >&2; exit 1; }
missing=()
for api in "${ALL_APIS[@]}"; do
  [ -f "$REPO_ROOT/services/$api/.env" ] || missing+=("$api")
done
if [ ${#missing[@]} -gt 0 ]; then
  echo "Missing .env file(s): ${missing[*]}. Copy services/<api>/.env.example to services/<api>/.env and edit it." >&2
  exit 1
fi

# 2. minikubeの起動(すでに動いていれば何もしない)
if ! minikube status >/dev/null 2>&1; then
  echo "Starting minikube..."
  minikube start --driver=docker
fi
# 自動スケール(HPA)がCPU使用率を取るために metrics-server が要る(すでに有効なら何も起きない)。
minikube addons enable metrics-server >/dev/null

# 3. ホストのDockerでビルドし、minikubeへ取り込む。
#    Deploymentは imagePullPolicy: Never なので、取り込まないとPodは ErrImageNeverPull になる。
#    同じタグ(:dev)を取り込み直すため、先にminikube内の古いイメージを消しておく。
if [ "$SKIP_BUILD" = false ]; then
  for api in "${APIS[@]}"; do
    image="pingucoin/$api:dev"
    echo "Building $image (host docker) ..."
    # コンテキストはリポジトリのルート(Go モジュールが1つで、pkg/ を共有しているため)。
    # バージョン・コミット・ビルド時刻を埋め込む(ログの version 属性や /version に出る)。
    docker build -t "$image" -f "$REPO_ROOT/services/$api/Dockerfile" \
      --build-arg VERSION="$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)" \
      --build-arg COMMIT="$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)" \
      --build-arg BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$REPO_ROOT"
    echo "Loading $image into minikube ..."
    minikube image rm "docker.io/$image" >/dev/null 2>&1 || true
    minikube image load "$image"
  done
fi

# 4. namespaceを作り、各APIの .env からSecretを作成(すでにあれば更新)
kubectl apply -f "$K8S_DIR/namespace.yaml"
for api in "${ALL_APIS[@]}"; do
  echo "Applying secret $api-env from services/$api/.env"
  (cd "$REPO_ROOT" && kubectl create secret generic "$api-env" -n "$NAMESPACE" \
    --from-env-file="services/$api/.env" --dry-run=client -o yaml) | kubectl apply -f -
done

# 5. マニフェストの適用。イメージ・Secretを更新した場合でも反映されるように再起動する。
kubectl apply -k "$K8S_DIR"
for api in "${APIS[@]}"; do
  kubectl rollout restart -n "$NAMESPACE" "deployment/$api"
done
for api in "${APIS[@]}"; do
  kubectl rollout status -n "$NAMESPACE" "deployment/$api" --timeout=180s
done
kubectl get pods -n "$NAMESPACE"

echo
echo "Ready. GraphQL: http://localhost:8082/api/v1/graphql (after port-forward)"

# 6. 必要ならpingu-apiをホストに公開(Ctrl+Cで終了)
if [ "$PORT_FORWARD" = true ]; then
  echo "Forwarding pingu-api to localhost:8082 (Ctrl+C to stop)..."
  kubectl port-forward -n "$NAMESPACE" service/pingu-api 8082:8082
else
  echo "To expose it: kubectl port-forward -n $NAMESPACE service/pingu-api 8082:8082"
fi
