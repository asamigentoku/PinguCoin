#!/usr/bin/env bash
# minikube上のAPIを停止(削除)するスクリプト。
#
#   bash script/stop.sh                  # API と Secret を削除
#   bash script/stop.sh --stop-minikube  # minikube 自体も停止
#
# namespace「pingucoin」ごと削除するので、Secretも消える。staging側のデータは削除されない。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
STOP_MINIKUBE=false
for arg in "$@"; do
  case "$arg" in
    --stop-minikube) STOP_MINIKUBE=true ;;
    *) echo "Unknown option: $arg" >&2; exit 1 ;;
  esac
done

kubectl delete -k "$REPO_ROOT/platform/kubernetes/minikube" --ignore-not-found
if [ "$STOP_MINIKUBE" = true ]; then
  minikube stop
fi
