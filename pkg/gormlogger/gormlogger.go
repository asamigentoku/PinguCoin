// Package gormlogger は、GORMのログを slog(JSON構造化ログ)に流し込むロガーを提供する。3つのサービスで共通に使う。
package gormlogger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
)

// slowQueryThreshold を超えたクエリは遅いクエリとしてWarnで出す。
const slowQueryThreshold = 200 * time.Millisecond

// slogGormLogger はGORMのクエリログを、main.goで設定したslog(JSON構造化ログ)に流し込むアダプタ。
// これを入れないと、GORMは独自の色付きテキストを標準出力に直接書き出してしまい、
// アプリ本体のJSONログと混在して読みにくくなる。
//
// また、gorm.ErrRecordNotFound(「1件も見つからなかった」)は
// internal/grpcserver 側で apperr.NotFound として正常に処理する想定のため、
// ここではエラーとしてログに出さない(単なる404相当をError扱いにしてログを汚さないため)。
type slogGormLogger struct {
	logger   *slog.Logger
	logLevel logger.LogLevel
}

// New はslogベースのGORM用ロガーを作る。
func New(slogLogger *slog.Logger) logger.Interface {
	return &slogGormLogger{
		logger:   slogLogger,
		logLevel: logger.Warn,
	}
}

func (gormLogger *slogGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *gormLogger
	newLogger.logLevel = level
	return &newLogger
}

func (gormLogger *slogGormLogger) Info(_ context.Context, msg string, args ...interface{}) {
	if gormLogger.logLevel >= logger.Info {
		gormLogger.logger.Info(fmt.Sprintf(msg, args...))
	}
}

func (gormLogger *slogGormLogger) Warn(_ context.Context, msg string, args ...interface{}) {
	if gormLogger.logLevel >= logger.Warn {
		gormLogger.logger.Warn(fmt.Sprintf(msg, args...))
	}
}

func (gormLogger *slogGormLogger) Error(_ context.Context, msg string, args ...interface{}) {
	if gormLogger.logLevel >= logger.Error {
		gormLogger.logger.Error(fmt.Sprintf(msg, args...))
	}
}

func (gormLogger *slogGormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if gormLogger.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && gormLogger.logLevel >= logger.Error:
		sql, rows := fc()
		gormLogger.logger.Error("gorm query failed",
			slog.Group("db", slog.String("statement", sql), slog.Int64("rows_affected", rows)),
			logging.Duration(elapsed),
			logging.Err(err),
		)
	case elapsed > slowQueryThreshold && gormLogger.logLevel >= logger.Warn:
		sql, rows := fc()
		gormLogger.logger.Warn("gorm slow query",
			slog.Group("db", slog.String("statement", sql), slog.Int64("rows_affected", rows)),
			logging.Duration(elapsed),
		)
	case gormLogger.logLevel >= logger.Info:
		sql, rows := fc()
		gormLogger.logger.Debug("gorm query",
			slog.Group("db", slog.String("statement", sql), slog.Int64("rows_affected", rows)),
			logging.Duration(elapsed),
		)
	}
}
