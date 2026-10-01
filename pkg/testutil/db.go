// Package testutil はテスト専用の補助部品。本番コード(cmd/, 各internalパッケージ)からは使わない。
package testutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseURLEnv は統合テスト用のPostgresの接続URLを渡す環境変数。
// 未設定のとき、DBを使うテストはスキップされる(単体テストだけが動く)。
//
//	docker run -d --name pg-test -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=pingucoin_test -p 55432:5432 postgres:16-alpine
//	TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55432/pingucoin_test?sslmode=disable" go test ./...
const DatabaseURLEnv = "TEST_DATABASE_URL"

// NewDB はテストごとに専用のスキーマを作り、そこに接続した *gorm.DB を返す
// (テスト同士が互いのデータに影響しない。終了時にスキーマごと削除する)。
// migrate にはそのサービスのマイグレーション関数(database.Migrate。本番と同じ SQL を適用する)を渡す。
func NewDB(t *testing.T, migrate func(*gorm.DB) error) *gorm.DB {
	t.Helper()
	base := os.Getenv(DatabaseURLEnv)
	if base == "" {
		t.Skipf("%s is not set; skipping the database integration test", DatabaseURLEnv)
	}

	quiet := &gorm.Config{Logger: logger.Discard}
	admin, err := gorm.Open(postgres.Open(base), quiet)
	if err != nil {
		t.Fatalf("connect to %s: %v", DatabaseURLEnv, err)
	}

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	schema := "t_" + hex.EncodeToString(suffix)
	if err := admin.Exec(`CREATE SCHEMA ` + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}

	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	db, err := gorm.Open(postgres.Open(parsed.String()), quiet)
	if err != nil {
		t.Fatalf("connect to schema %s: %v", schema, err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if migrate != nil {
		if err := migrate(db); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db
}

// FakeDB はPostgresなしで *gorm.DB を作るための偽のDB。Ping(疎通確認)の成否をテストから切り替えられる。
// ヘルスチェックのように「DBが落ちたら?」を試したいテストで使う。
type FakeDB struct {
	*gorm.DB
	failing atomic.Bool
}

// SetFailing を true にすると、以降の Ping はエラーになる(DBが落ちた状態)。
func (fake *FakeDB) SetFailing(failing bool) { fake.failing.Store(failing) }

var fakeDriverSeq atomic.Int64

// NewFakeDB は、Ping だけができる偽のDBに繋いだ *gorm.DB を返す(クエリは実行できない)。
func NewFakeDB(t *testing.T) *FakeDB {
	t.Helper()
	fake := &FakeDB{}
	name := fmt.Sprintf("testutil-fake-%d", fakeDriverSeq.Add(1))
	sql.Register(name, &fakeDriver{fake: fake})
	sqlDB, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	fake.DB = db
	return fake
}

type fakeDriver struct{ fake *FakeDB }

func (d *fakeDriver) Open(string) (driver.Conn, error) { return &fakeConn{fake: d.fake}, nil }

type fakeConn struct{ fake *FakeDB }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake db: queries are not supported")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fake db: transactions are not supported")
}

// Ping は database/sql が呼ぶ疎通確認(driver.Pinger)。
func (c *fakeConn) Ping(context.Context) error {
	if c.fake.failing.Load() {
		return errors.New("fake db: connection refused")
	}
	return nil
}
