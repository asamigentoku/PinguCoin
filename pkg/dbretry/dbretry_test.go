package dbretry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/retry"
)

// 待たずに、すぐ再試行する方針(テストを速くするため)。
var fast = retry.Policy{MaxAttempts: 4, InitialBackoff: time.Microsecond, MaxBackoff: time.Microsecond, Multiplier: 1}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestIsPermanent(t *testing.T) {
	_, parseErr := pgconn.ParseConfig("postgres://user:pass@host:notaport/db")
	if parseErr == nil {
		t.Fatal("expected ParseConfig to fail")
	}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unparsable connection string", parseErr, true},
		{"wrapped unparsable connection string", fmt.Errorf("failed to connect database: %w", parseErr), true},
		{"wrong password", &pgconn.PgError{Code: "28P01"}, true},
		{"invalid authorization", &pgconn.PgError{Code: "28000"}, true},
		{"database does not exist", &pgconn.PgError{Code: "3D000"}, true},
		{"server is starting up", &pgconn.PgError{Code: "57P03"}, false},
		{"too many connections", &pgconn.PgError{Code: "53300"}, false},
		{"connection refused", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, false},
		{"timeout", context.DeadlineExceeded, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tt := range tests {
		if got := IsPermanent(tt.err); got != tt.want {
			t.Errorf("%s: IsPermanent = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// DB が一時的につながらないときは、つながるまで、(上限の範囲で)再試行する。
func TestRetriesWhileTheDatabaseIsStartingUp(t *testing.T) {
	calls := 0
	want := &gorm.DB{}

	db, err := connect(context.Background(), quiet(), fast, func() (*gorm.DB, error) {
		calls++
		if calls < 3 {
			return nil, &pgconn.PgError{Code: "57P03", Message: "the database system is starting up"}
		}
		return want, nil
	})

	if err != nil || db != want || calls != 3 {
		t.Errorf("db=%v err=%v calls=%d, want the third attempt to succeed", db, err, calls)
	}
}

// 設定の間違い(パスワードが違うなど)は、すぐに諦める。待っても直らない。
func TestConfigurationErrorsFailImmediately(t *testing.T) {
	calls := 0
	wrongPassword := &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}

	_, err := connect(context.Background(), quiet(), fast, func() (*gorm.DB, error) {
		calls++
		return nil, fmt.Errorf("failed to connect database: %w", wrongPassword)
	})

	if calls != 1 {
		t.Errorf("attempts = %d, want 1 (a configuration error must not be retried)", calls)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "28P01" {
		t.Errorf("err = %v, want the original error", err)
	}
}

// ずっとつながらなければ、上限の回数で諦めて、最後のエラーを返す。
func TestGivesUpWhenTheDatabaseNeverComesUp(t *testing.T) {
	calls := 0
	refused := &net.OpError{Op: "dial", Err: errors.New("connection refused")}

	_, err := connect(context.Background(), quiet(), fast, func() (*gorm.DB, error) {
		calls++
		return nil, refused
	})

	if calls != fast.MaxAttempts {
		t.Errorf("attempts = %d, want %d", calls, fast.MaxAttempts)
	}
	if !errors.Is(err, refused) {
		t.Errorf("err = %v, want the last error", err)
	}
}

// 既定の方針(retry.Startup)でも、設定の間違いは、待たずに失敗する。
func TestConnectUsesTheStartupPolicy(t *testing.T) {
	begin := time.Now()
	_, err := Connect(context.Background(), quiet(), func() (*gorm.DB, error) {
		return nil, &pgconn.PgError{Code: "3D000"}
	})
	if err == nil || time.Since(begin) > time.Second {
		t.Errorf("err=%v after %v; a missing database should fail immediately", err, time.Since(begin))
	}
}
