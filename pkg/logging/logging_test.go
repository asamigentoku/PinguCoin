package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/asamigentoku/PinguCoin/pkg/requestid"
)

func logOnce(t *testing.T, config Config, emit func(*slog.Logger)) map[string]any {
	t.Helper()
	var buffer bytes.Buffer
	config.Writer = &buffer
	emit(New(config))

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &line); err != nil {
		t.Fatalf("a log entry must be one JSON object: %v\n%s", err, buffer.String())
	}
	return line
}

// すべてのログに、Datadog の予約属性(timestamp / status / message / service / env / version)が付く。
func TestEveryLogCarriesTheReservedAttributes(t *testing.T) {
	before := time.Now().Add(-time.Second)
	line := logOnce(t, Config{Service: "orcan-api", Env: "production", Version: "v1.2.3"}, func(l *slog.Logger) {
		l.Info("order created")
	})

	want := map[string]string{"status": "info", "message": "order created", "service": "orcan-api", "env": "production", "version": "v1.2.3"}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("%s = %v, want %q", key, line[key], value)
		}
	}
	// 組み込みの名前(time / level / msg)は、残らない。
	for _, old := range []string{"time", "level", "msg"} {
		if _, found := line[old]; found {
			t.Errorf("the built-in key %q should have been renamed", old)
		}
	}
	timestamp, err := time.Parse(time.RFC3339Nano, fmt.Sprint(line["timestamp"]))
	if err != nil || timestamp.Before(before) {
		t.Errorf("timestamp = %v (err=%v), want an RFC 3339 time", line["timestamp"], err)
	}
	// どの環境でも UTC(末尾が Z)。開発のPCのローカル時刻(+09:00 など)では出さない。
	if !strings.HasSuffix(fmt.Sprint(line["timestamp"]), "Z") {
		t.Errorf("timestamp %v must be in UTC (ending with Z)", line["timestamp"])
	}
}

func TestStatusIsLowercaseAndFollowsDatadogNames(t *testing.T) {
	for level, want := range map[slog.Level]string{slog.LevelDebug: "debug", slog.LevelInfo: "info", slog.LevelWarn: "warn", slog.LevelError: "error"} {
		line := logOnce(t, Config{Service: "s", Level: slog.LevelDebug}, func(l *slog.Logger) { l.Log(context.Background(), level, "m") })
		if line["status"] != want {
			t.Errorf("level %v: status = %v, want %q", level, line["status"], want)
		}
	}
}

func TestDefaultsForEnvAndVersion(t *testing.T) {
	line := logOnce(t, Config{Service: "s"}, func(l *slog.Logger) { l.Info("m") })

	if line["env"] != "local" {
		t.Errorf("env = %v, want local", line["env"])
	}
	if line["version"] == "" || line["version"] == nil {
		t.Error("version must never be empty (falls back to pkg/version)")
	}
}

func TestLevelFiltersLogs(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(Config{Service: "s", Level: slog.LevelWarn, Writer: &buffer})

	logger.Info("hidden")
	logger.Debug("hidden")
	logger.Warn("shown")

	if lines := strings.Count(strings.TrimSpace(buffer.String()), "\n") + 1; lines != 1 || !strings.Contains(buffer.String(), "shown") {
		t.Errorf("only the warning should be logged: %s", buffer.String())
	}
}

// 入れ子のグループの中の time / level / msg という名前は、変えない(例: http グループの中の level)。
func TestNestedKeysAreNotRenamed(t *testing.T) {
	line := logOnce(t, Config{Service: "s"}, func(l *slog.Logger) {
		l.Info("m", slog.Group("custom", slog.String("level", "x"), slog.String("time", "y"), slog.String("msg", "z")))
	})

	custom, _ := line["custom"].(map[string]any)
	if custom["level"] != "x" || custom["time"] != "y" || custom["msg"] != "z" {
		t.Errorf("nested keys were renamed: %v", custom)
	}
}

// Err: Datadog の標準属性 error.kind / error.message。
func TestErrAttributes(t *testing.T) {
	cause := os.ErrNotExist
	line := logOnce(t, Config{Service: "s"}, func(l *slog.Logger) {
		l.Error("failed", Err(fmt.Errorf("open config: %w", cause)))
	})

	group, _ := line["error"].(map[string]any)
	if group["message"] != "open config: file does not exist" {
		t.Errorf("error.message = %v", group["message"])
	}
	// kind は、ラップした側(*fmt.wrapError)ではなく、根っこの原因の型。
	if group["kind"] != "*errors.errorString" && group["kind"] != fmt.Sprintf("%T", cause) {
		t.Errorf("error.kind = %v, want the root cause's type %T", group["kind"], cause)
	}
}

func TestErrOfNilIsOmitted(t *testing.T) {
	line := logOnce(t, Config{Service: "s"}, func(l *slog.Logger) { l.Info("ok", Err(nil)) })
	if _, found := line["error"]; found {
		t.Errorf("a nil error must not add an error attribute: %v", line["error"])
	}
	if !errors.Is(os.ErrNotExist, os.ErrNotExist) {
		t.Fatal("sanity")
	}
}

// duration は、ナノ秒の整数(Datadog の duration の単位)。
func TestDurationIsInNanoseconds(t *testing.T) {
	line := logOnce(t, Config{Service: "s"}, func(l *slog.Logger) { l.Info("m", Duration(1500*time.Microsecond)) })

	if line["duration"] != float64(1_500_000) {
		t.Errorf("duration = %v, want 1500000 (ns)", line["duration"])
	}
}

// request_id: サービスをまたいで追うためのキー。ID が無いときは、出さない。
func TestRequestIDAttribute(t *testing.T) {
	ctx := requestid.WithContext(context.Background(), "req-123")
	line := logOnce(t, Config{Service: "s"}, func(l *slog.Logger) { l.InfoContext(ctx, "m", RequestID(ctx)) })
	if line["request_id"] != "req-123" {
		t.Errorf("request_id = %v", line["request_id"])
	}

	line = logOnce(t, Config{Service: "s"}, func(l *slog.Logger) { l.Info("m", RequestID(context.Background())) })
	if _, found := line["request_id"]; found {
		t.Error("request_id must be omitted when there is no ID")
	}
}

func TestParseLevel(t *testing.T) {
	for value, want := range map[string]slog.Level{
		"debug": slog.LevelDebug, "DEBUG": slog.LevelDebug, " warn ": slog.LevelWarn, "warning": slog.LevelWarn,
		"error": slog.LevelError, "info": slog.LevelInfo, "": slog.LevelInfo, "nonsense": slog.LevelInfo,
	} {
		if got := ParseLevel(value); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", value, got, want)
		}
	}
}

// DD_ENV を優先する。無ければ APP_ENV。どちらも無ければ空(ログでは local になる)。
func TestEnvFromEnvironment(t *testing.T) {
	t.Setenv("DD_ENV", "")
	t.Setenv("APP_ENV", "")
	if got := EnvFromEnvironment(); got != "" {
		t.Errorf("EnvFromEnvironment() = %q, want empty", got)
	}

	t.Setenv("APP_ENV", "staging")
	if got := EnvFromEnvironment(); got != "staging" {
		t.Errorf("APP_ENV should be used when DD_ENV is not set: %q", got)
	}

	t.Setenv("DD_ENV", " production ")
	if got := EnvFromEnvironment(); got != "production" {
		t.Errorf("DD_ENV should win over APP_ENV (and be trimmed): %q", got)
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("LOG_LEVEL", "error")
	var buffer bytes.Buffer
	// NewFromEnv は標準出力に出すので、同じ設定の New で、環境変数の読み取りを確かめる。
	logger := New(Config{Service: "pingu-api", Env: os.Getenv("APP_ENV"), Level: ParseLevel(os.Getenv("LOG_LEVEL")), Writer: &buffer})

	logger.Warn("hidden")
	logger.Error("shown")

	if !strings.Contains(buffer.String(), `"env":"staging"`) || strings.Contains(buffer.String(), "hidden") {
		t.Errorf("unexpected output: %s", buffer.String())
	}
	if NewFromEnv("pingu-api") == nil {
		t.Error("NewFromEnv returned nil")
	}
}
