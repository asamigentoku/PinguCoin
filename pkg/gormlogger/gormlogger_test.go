package gormlogger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
)

// Datadog の標準属性にそろえた、1行分のログ(db.statement / error.message / duration)。
type entry struct {
	Level string `json:"status"`
	Msg   string `json:"message"`
	DB    struct {
		Statement    string `json:"statement"`
		RowsAffected int64  `json:"rows_affected"`
	} `json:"db"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Duration *int64 `json:"duration"`
}

func newLogger(buffer *bytes.Buffer, level logger.LogLevel) logger.Interface {
	// DEBUG まで出して、どのレベルで出たかを確かめる。
	slogLogger := logging.New(logging.Config{Service: "test", Level: slog.LevelDebug, Writer: buffer})
	return New(slogLogger).LogMode(level)
}

func entries(t *testing.T, buffer *bytes.Buffer) []entry {
	t.Helper()
	var result []entry
	for _, raw := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
		if raw == "" {
			continue
		}
		var line entry
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("log line is not JSON: %q", raw)
		}
		result = append(result, line)
	}
	return result
}

func query() (string, int64) { return "SELECT 1", 1 }

// 既定のレベルは Warn。普通のクエリのログは出さない(ログを埋めないため)。
func TestDefaultLevelHidesNormalQueries(t *testing.T) {
	var buffer bytes.Buffer
	l := New(logging.New(logging.Config{Service: "test", Level: slog.LevelDebug, Writer: &buffer}))

	l.Trace(context.Background(), time.Now(), query, nil)
	l.Info(context.Background(), "info %s", "x")

	if buffer.Len() != 0 {
		t.Errorf("the default logger should be quiet, got %s", buffer.String())
	}
}

func TestMessagesAreFormattedAndFilteredByLevel(t *testing.T) {
	tests := []struct {
		level logger.LogLevel
		want  []string // 出るメッセージ
	}{
		{logger.Silent, nil},
		{logger.Error, []string{"error 1"}},
		{logger.Warn, []string{"warn 1", "error 1"}},
		{logger.Info, []string{"info 1", "warn 1", "error 1"}},
	}
	for _, tt := range tests {
		var buffer bytes.Buffer
		l := newLogger(&buffer, tt.level)
		l.Info(context.Background(), "info %d", 1)
		l.Warn(context.Background(), "warn %d", 1)
		l.Error(context.Background(), "error %d", 1)

		var got []string
		for _, line := range entries(t, &buffer) {
			got = append(got, line.Msg)
		}
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("level %v logged %v, want %v", tt.level, got, tt.want)
		}
	}
}

func TestFailedQueriesAreLoggedAsErrors(t *testing.T) {
	var buffer bytes.Buffer
	l := newLogger(&buffer, logger.Warn)

	l.Trace(context.Background(), time.Now(), query, errors.New("duplicate key"))

	got := entries(t, &buffer)
	if len(got) != 1 || got[0].Level != "error" || got[0].Msg != "gorm query failed" || got[0].DB.Statement != "SELECT 1" || got[0].DB.RowsAffected != 1 || got[0].Error.Message != "duplicate key" || got[0].Duration == nil {
		t.Errorf("unexpected log: %+v", got)
	}
}

// 「1件も見つからなかった」は、アプリ側で NotFound として正常に扱うので、エラーとしてログに出さない。
func TestRecordNotFoundIsNotLoggedAsAnError(t *testing.T) {
	var buffer bytes.Buffer
	l := newLogger(&buffer, logger.Warn)

	l.Trace(context.Background(), time.Now(), query, gorm.ErrRecordNotFound)

	if buffer.Len() != 0 {
		t.Errorf("ErrRecordNotFound must not be logged: %s", buffer.String())
	}
}

func TestSlowQueriesAreLoggedAsWarnings(t *testing.T) {
	var buffer bytes.Buffer
	l := newLogger(&buffer, logger.Warn)

	l.Trace(context.Background(), time.Now().Add(-time.Second), query, nil)

	got := entries(t, &buffer)
	if len(got) != 1 || got[0].Level != "warn" || got[0].Msg != "gorm slow query" {
		t.Errorf("unexpected log: %+v", got)
	}
}

func TestInfoLevelLogsEveryQueryAsDebug(t *testing.T) {
	var buffer bytes.Buffer
	l := newLogger(&buffer, logger.Info)

	l.Trace(context.Background(), time.Now(), query, nil)

	got := entries(t, &buffer)
	if len(got) != 1 || got[0].Level != "debug" || got[0].Msg != "gorm query" {
		t.Errorf("unexpected log: %+v", got)
	}
}

func TestSilentLoggerLogsNothing(t *testing.T) {
	var buffer bytes.Buffer
	l := newLogger(&buffer, logger.Silent)

	l.Trace(context.Background(), time.Now().Add(-time.Second), query, errors.New("boom"))

	if buffer.Len() != 0 {
		t.Errorf("a silent logger logged: %s", buffer.String())
	}
}

// LogMode は元のロガーを書き換えず、新しいロガーを返す。
func TestLogModeDoesNotModifyTheOriginal(t *testing.T) {
	var buffer bytes.Buffer
	original := New(logging.New(logging.Config{Service: "test", Writer: &buffer}))

	_ = original.LogMode(logger.Silent)
	original.Warn(context.Background(), "still on")

	if len(entries(t, &buffer)) != 1 {
		t.Error("LogMode changed the original logger")
	}
}
