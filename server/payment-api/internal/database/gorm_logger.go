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
// gorm.ErrRecordNotFound は internal/grpcserver 側で apperr.NotFound として正常に処理する想定のため、
// ここではエラーとしてログに出さない。
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

func (l *slogGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

func (l *slogGormLogger) Info(_ context.Context, msg string, args ...interface{}) {
	if l.logLevel >= logger.Info {
		l.logger.Info(fmt.Sprintf(msg, args...))
	}
}

func (l *slogGormLogger) Warn(_ context.Context, msg string, args ...interface{}) {
	if l.logLevel >= logger.Warn {
		l.logger.Warn(fmt.Sprintf(msg, args...))
	}
}

func (l *slogGormLogger) Error(_ context.Context, msg string, args ...interface{}) {
	if l.logLevel >= logger.Error {
		l.logger.Error(fmt.Sprintf(msg, args...))
	}
}

func (l *slogGormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && l.logLevel >= logger.Error:
		sql, rows := fc()
		l.logger.Error("gorm query failed",
			slog.String("sql", sql),
			slog.Int64("rows", rows),
			slog.Duration("elapsed", elapsed),
			slog.String("error", err.Error()),
		)
	case elapsed > slowQueryThreshold && l.logLevel >= logger.Warn:
		sql, rows := fc()
		l.logger.Warn("gorm slow query",
			slog.String("sql", sql),
			slog.Int64("rows", rows),
			slog.Duration("elapsed", elapsed),
		)
	case l.logLevel >= logger.Info:
		sql, rows := fc()
		l.logger.Debug("gorm query",
			slog.String("sql", sql),
			slog.Int64("rows", rows),
			slog.Duration("elapsed", elapsed),
		)
	}
}
