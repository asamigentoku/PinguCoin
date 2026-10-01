// Package logging は、3つのサービスのログを、同じ形式(Datadog の標準属性)にそろえる。
//
// ログは、1行が1つの JSON(標準出力)。属性の名前は、サービスごとにばらばらにせず、次にそろえる。
// そろえておくと、ログ管理(Datadog)で、サービスをまたいで、同じ条件で検索・集計できる
// (例: 全サービスの http.status_code が 500 のログ、request_id でのリクエストの追跡)。
//
// 予約された属性(Datadog が、意味を決めているもの):
//
//	timestamp   ログの時刻(ISO 8601 / RFC 3339、UTC)
//	status      ログのレベル(debug / info / warn / error)
//	message     ログの本文
//	service     サービス名(orcan-api / payment-api / pingu-api)
//	env         環境(local / staging / production。環境変数 DD_ENV。無ければ APP_ENV)
//	version     サービスのバージョン(pkg/version)
//
// 標準の属性(Datadog の「標準属性」)。JSON の入れ子(グループ)は、`http.method` のように、ドットでつながって見える:
//
//	http.method / http.url / http.status_code / http.useragent / http.referer
//	network.client.ip        クライアントの IP
//	duration                 処理時間(ナノ秒の整数。Datadog の「duration」の単位)
//	error.kind / error.message   エラーの種類と本文(Err を使う)
//	db.statement / db.rows_affected   DB のクエリ(pkg/gormlogger)
//
// 追跡のための属性:
//
//	request_id               リクエスト ID(pkg/requestid)。サービスをまたいで、同じ値が出る
//
// gRPC は、OpenTelemetry の命名に合わせる:
//
//	rpc.system = "grpc" / rpc.service / rpc.method / rpc.grpc.status_code
//
// 秘密(パスワード、トークン、接続文字列、リクエストの本文)は、ログに出さない。
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/asamigentoku/PinguCoin/pkg/requestid"
	"github.com/asamigentoku/PinguCoin/pkg/version"
)

// Config は、ロガーの設定。
type Config struct {
	// Service は、サービス名(必須)。
	Service string
	// Env は、環境(local / staging / production)。
	Env string
	// Version は、サービスのバージョン。空なら、pkg/version のバージョン。
	Version string
	// Level は、出すログの最低レベル。nil なら、info。
	Level slog.Leveler
	// Writer は、出力先。nil なら、標準出力。
	Writer io.Writer
}

// New は、Datadog の標準属性にそろえた、JSON のロガーを作る。
// すべてのログに、service / env / version が付く。
func New(config Config) *slog.Logger {
	writer := config.Writer
	if writer == nil {
		writer = os.Stdout
	}
	var level slog.Leveler = slog.LevelInfo
	if config.Level != nil {
		level = config.Level
	}
	appVersion := config.Version
	if appVersion == "" {
		appVersion = version.Get().Version
	}
	env := config.Env
	if env == "" {
		env = "local"
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: renameBuiltins,
	})
	return slog.New(handler).With(
		slog.String("service", config.Service),
		slog.String("env", env),
		slog.String("version", appVersion),
	)
}

// NewFromEnv は、環境変数から設定して、ロガーを作る。
//
//	DD_ENV     環境(Datadog の慣習の名前。ConfigMap で、全サービスにそろえて渡す)
//	APP_ENV    DD_ENV が無いときの、環境(orcan-api では、Blob のパスの環境名にも使われる)
//	LOG_LEVEL  debug / info / warn / error(既定 info)
//
// 環境が、どちらにも無ければ local。
func NewFromEnv(service string) *slog.Logger {
	return New(Config{
		Service: service,
		Env:     EnvFromEnvironment(),
		Level:   ParseLevel(os.Getenv("LOG_LEVEL")),
	})
}

// EnvFromEnvironment は、ログの env 属性にする環境名。DD_ENV、APP_ENV の順で使う。どちらも無ければ空(New が local にする)。
func EnvFromEnvironment() string {
	if env := strings.TrimSpace(os.Getenv("DD_ENV")); env != "" {
		return env
	}
	return strings.TrimSpace(os.Getenv("APP_ENV"))
}

// renameBuiltins は、slog の組み込みの属性名を、Datadog の予約属性の名前に変える(トップレベルだけ)。
func renameBuiltins(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return attr
	}
	switch attr.Key {
	case slog.TimeKey:
		attr.Key = "timestamp"
		// どの環境(開発のPCの日本時間、コンテナの UTC)でも、同じ形式(UTC、末尾が Z)にそろえる。
		if t, ok := attr.Value.Any().(time.Time); ok {
			attr.Value = slog.TimeValue(t.UTC())
		}
	case slog.MessageKey:
		attr.Key = "message"
	case slog.LevelKey:
		attr.Key = "status"
		if level, ok := attr.Value.Any().(slog.Level); ok {
			attr.Value = slog.StringValue(statusOf(level))
		}
	}
	return attr
}

// statusOf は、slog のレベルを、Datadog の status(小文字)にする。
func statusOf(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "error"
	case level >= slog.LevelWarn:
		return "warn"
	case level >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

// ParseLevel は、LOG_LEVEL の文字列を、レベルにする。分からなければ info。
func ParseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Err は、エラーを、Datadog の標準属性(error.kind / error.message)にする。
// kind は、エラーの根っこ(errors.Unwrap をたどった先)の型。err が nil なら、何も出さない。
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	root := err
	for {
		next := errors.Unwrap(root)
		if next == nil {
			break
		}
		root = next
	}
	return slog.Group("error",
		slog.String("kind", fmt.Sprintf("%T", root)),
		slog.String("message", err.Error()),
	)
}

// Duration は、処理時間を、Datadog の duration(ナノ秒の整数)にする。
func Duration(d time.Duration) slog.Attr {
	return slog.Int64("duration", d.Nanoseconds())
}

// RequestID は、context のリクエスト ID を、request_id 属性にする。ID が無ければ、何も出さない。
func RequestID(ctx context.Context) slog.Attr {
	id := requestid.FromContext(ctx)
	if id == "" {
		return slog.Attr{}
	}
	return slog.String("request_id", id)
}
