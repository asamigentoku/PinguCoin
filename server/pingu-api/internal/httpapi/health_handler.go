package httpapi

import (
	"context"
	"net/http"
	"time"
)

// HealthHandler はKubernetesのprobe用のエンドポイントを提供する。
//
//	GET /healthz ... liveness。プロセスが応答できれば常に200(依存先の状態は見ない)
//	GET /readyz  ... readiness。DBに接続できる間だけ200、できなければ503(Serviceの宛先から外れる)
//
// livenessにDBを含めないのは、DBが一時的に落ちただけでPodを再起動し続けないため。
// どちらも認証不要で、ログにも出さない(WithLoggingが対象外にしている)。
type HealthHandler struct {
	ping func(context.Context) error
}

func NewHealthHandler(ping func(context.Context) error) *HealthHandler {
	return &HealthHandler{ping: ping}
}

func (handler *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := handler.ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "reason": "database unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// isHealthCheck はprobe用のパスか。数秒おきに呼ばれてログを埋めないよう、ログの対象外にする。
func isHealthCheck(path string) bool {
	return path == "/healthz" || path == "/readyz"
}
