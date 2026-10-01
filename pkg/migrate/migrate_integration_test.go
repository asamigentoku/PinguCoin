package migrate_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/migrate"
	"github.com/asamigentoku/PinguCoin/pkg/testutil"
)

// 実際のPostgresが必要(TEST_DATABASE_URL)。アドバイザリロックはDB自身の機能なので、DBで確かめる。

// 複数の Pod が同時にマイグレーションしても、1つずつ実行される(重ならない)。
func TestMigrationsNeverOverlap(t *testing.T) {
	db := testutil.NewDB(t, nil)

	var running, maxRunning, runs atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := migrate.WithLock(context.Background(), db, func(*gorm.DB) error {
				now := running.Add(1)
				for {
					seen := maxRunning.Load()
					if now <= seen || maxRunning.CompareAndSwap(seen, now) {
						break
					}
				}
				time.Sleep(30 * time.Millisecond) // 重なるなら、この間に重なる
				running.Add(-1)
				runs.Add(1)
				return nil
			})
			if err != nil {
				t.Errorf("WithLock: %v", err)
			}
		}()
	}
	wg.Wait()

	if maxRunning.Load() != 1 {
		t.Errorf("%d migrations ran at the same time, want 1", maxRunning.Load())
	}
	if runs.Load() != 6 {
		t.Errorf("%d migrations ran, want 6 (each waits for its turn)", runs.Load())
	}
}

// fn が失敗しても、ロックは解放され、次の Pod が実行できる。
func TestLockIsReleasedWhenTheMigrationFails(t *testing.T) {
	db := testutil.NewDB(t, nil)
	boom := errors.New("migration failed")

	if err := migrate.WithLock(context.Background(), db, func(*gorm.DB) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the migration's own error", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- migrate.WithLock(context.Background(), db, func(*gorm.DB) error { return nil })
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the next migration failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the lock was not released after a failed migration")
	}
}

// fn の中でも、同じ接続が使われる(ロックを取った接続で、マイグレーションが走る)。
func TestFunctionRunsOnTheLockedConnection(t *testing.T) {
	db := testutil.NewDB(t, nil)

	err := migrate.WithLock(context.Background(), db, func(conn *gorm.DB) error {
		var held bool
		// このセッションがロックを持っていれば、pg_locks に、自分のpidで載っている。
		return conn.Raw(`SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND pid = pg_backend_pid() AND granted)`).Scan(&held).Error
	})
	if err != nil {
		t.Fatal(err)
	}
}
