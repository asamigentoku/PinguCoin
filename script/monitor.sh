#!/usr/bin/env bash
# 監視の画面(Grafana と Prometheus)を、手元のブラウザで開けるようにする(port-forward。Ctrl+C で終了)。
# minikube でも、本番(AKS。az aks get-credentials 済みの、管理者)でも、同じように動く(いまの kubectl の接続先に対して)。
#
#   bash script/monitor.sh            # Grafana → http://localhost:3001 、Prometheus → http://localhost:9090
#   NAMESPACE=pingucoin GRAFANA_PORT=3001 PROMETHEUS_PORT=9090 bash script/monitor.sh
#
# Grafana のポートを 3001 にしているのは、フロントエンド(next dev)が 3000 を使うため。
# 本番の Grafana のログインは、ユーザー admin と、Key Vault の grafana-admin-password(下の例で取れる)。
#   az keyvault secret show --vault-name <key-vault-name> --name grafana-admin-password --query value -o tsv
set -euo pipefail

NAMESPACE="${NAMESPACE:-pingucoin}"
GRAFANA_PORT="${GRAFANA_PORT:-3001}"
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9090}"

command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed or not in PATH." >&2; exit 1; }
kubectl get deployment/grafana deployment/prometheus -n "$NAMESPACE" >/dev/null \
  || { echo "monitoring is not deployed in namespace '$NAMESPACE' (bash script/start.sh, or the production deploy)." >&2; exit 1; }

pids=()
cleanup() { for pid in "${pids[@]:-}"; do kill "$pid" 2>/dev/null || true; done; }
trap cleanup EXIT INT TERM

kubectl port-forward -n "$NAMESPACE" svc/grafana "$GRAFANA_PORT:3000" >/dev/null & pids+=($!)
kubectl port-forward -n "$NAMESPACE" svc/prometheus "$PROMETHEUS_PORT:9090" >/dev/null & pids+=($!)

echo "Grafana    : http://localhost:$GRAFANA_PORT   (dashboards folder: PinguCoin)"
echo "Prometheus : http://localhost:$PROMETHEUS_PORT   (alerts: /alerts, targets: /targets)"
echo "Press Ctrl+C to stop."
wait
