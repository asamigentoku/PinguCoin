package interceptor

import "strings"

// healthMethodPrefix はgRPC標準のヘルスチェック(grpc.health.v1.Health)のメソッド名の接頭辞。
// kubeletのprobeはサービス間認証用のトークンを持たないため、認証の対象外にする。
// また、数秒おきに呼ばれてログを埋めないよう、ログの対象外にもする。
// 返すのは「動いているか」だけで、業務データは含まない。
const healthMethodPrefix = "/grpc.health.v1.Health/"

func isHealthCheck(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, healthMethodPrefix)
}
