#!/usr/bin/env bash
# minikube上のAPIを停止(削除)するスクリプト。
#
#   bash infrastructure/kubernetes/minikube/stop.sh                  # API と Secret を削除
#   bash infrastructure/kubernetes/minikube/stop.sh --stop-minikube  # minikube 自体も停止
#
# namespace「pingucoin」ごと削除するので、Secretも消える。staging側のデータは削除されない。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STOP_MINIKUBE=false
for arg in "$@"; do
  case "$arg" in
    --stop-minikube) STOP_MINIKUBE=true ;;
    *) echo "Unknown option: $arg" >&2; exit 1 ;;
  esac
done

kubectl delete -k "$SCRIPT_DIR" --ignore-not-found
if [ "$STOP_MINIKUBE" = true ]; then
  minikube stop
fi
