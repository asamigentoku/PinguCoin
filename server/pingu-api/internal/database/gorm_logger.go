package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

// NewGormLogger はslogベースのGORM用ロガーを作る。
func NewGormLogger(l *slog.Logger) logger.Interface {
	return &slogGormLogger{
		logger:   l,
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

func (gormLogger *slogGormLogger) Trace(_ context.Context, begin time.Time, sqlFn func() (string, int64), err error) {
	if gormLogger.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && gormLogger.logLevel >= logger.Error:
		sql, rows := sqlFn()
		gormLogger.logger.Error("gorm query failed",
			slog.String("sql", sql),
			slog.Int64("rows", rows),
			slog.Duration("elapsed", elapsed),
			slog.String("error", err.Error()),
		)
	case elapsed > slowQueryThreshold && gormLogger.logLevel >= logger.Warn:
		sql, rows := sqlFn()
		gormLogger.logger.Warn("gorm slow query",
			slog.String("sql", sql),
			slog.Int64("rows", rows),
			slog.Duration("elapsed", elapsed),
		)
	case gormLogger.logLevel >= logger.Info:
		sql, rows := sqlFn()
		gormLogger.logger.Debug("gorm query",
			slog.String("sql", sql),
			slog.Int64("rows", rows),
			slog.Duration("elapsed", elapsed),
		)
	}
}
