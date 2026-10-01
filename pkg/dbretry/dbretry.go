// Package dbretry は、起動時の DB への接続を、一時的な失敗のときだけ、バックオフつきで再試行する。
//
// DB が、起動中・再起動中・メンテナンス中などで、一時的につながらないことはよくある。そのたびにプロセスが落ちて、
// Kubernetes に再起動されるのを待つより、数秒待って、つなぎ直すほうが早く、安定する。
//
// ただし、「何でも再試行」はしない。設定の間違い(接続文字列が不正、パスワードが違う、データベースが存在しない)は、
// 何度やっても同じなので、すぐに諦めて、原因が分かるエラーで終了する。
package dbretry

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/retry"
)

// Connect は、open(DB へ接続する関数)を、一時的な失敗のときだけ、retry.Startup の方針で再試行する。
func Connect(ctx context.Context, logger *slog.Logger, open func() (*gorm.DB, error)) (*gorm.DB, error) {
	return connect(ctx, logger, retry.Startup, open)
}

func connect(ctx context.Context, logger *slog.Logger, policy retry.Policy, open func() (*gorm.DB, error)) (*gorm.DB, error) {
	return retry.DoValue(ctx, policy, func(context.Context) (*gorm.DB, error) {
		db, err := open()
		if err != nil && IsPermanent(err) {
			return nil, retry.Permanent(err)
		}
		return db, err
	}, retry.OnRetry(func(attempt int, err error, wait time.Duration) {
		logger.Warn("database is not ready; retrying",
			slog.Int("attempt", attempt),
			slog.Int64("retry_in_ms", wait.Milliseconds()),
			logging.Err(err),
		)
	}))
}

// IsPermanent は、再試行しても直らない接続のエラーか(設定の間違い)。
//   - 接続文字列を解釈できない
//   - 認証に失敗した(ユーザー名・パスワードが違う): SQLSTATE クラス 28
//   - データベースが存在しない: SQLSTATE 3D000
//
// 「サーバーが起動中」(57P03)、「接続が多すぎる」(53300)、ネットワークの断・タイムアウトなどは、一時的なので、含まない。
func IsPermanent(err error) bool {
	var parseErr *pgconn.ParseConfigError
	if errors.As(err, &parseErr) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "28") || pgErr.Code == "3D000"
	}
	return false
}
