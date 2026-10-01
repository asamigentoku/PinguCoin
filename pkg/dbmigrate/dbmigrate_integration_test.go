package dbmigrate_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/dbmigrate"
	"github.com/asamigentoku/PinguCoin/pkg/testutil"
)

// 実際のPostgresが必要(TEST_DATABASE_URL)。トランザクション・ロック・チェックサムは、DB の機能なので、DB で確かめる。

func load(t *testing.T, entries map[string]string) []dbmigrate.Migration {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, content := range entries {
		fsys["m/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	migrations, err := dbmigrate.Load(fsys, "m")
	if err != nil {
		t.Fatal(err)
	}
	return migrations
}

var base = map[string]string{
	"000001_baseline.sql": "CREATE TABLE widgets (id serial PRIMARY KEY, name text NOT NULL);",
	"000002_seed.sql":     "INSERT INTO widgets (name) VALUES ('first'), ('second');",
	"000003_column.sql":   "ALTER TABLE widgets ADD COLUMN color text NOT NULL DEFAULT 'red';",
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	if err := db.Table(table).Count(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestAppliesEverythingToANewDatabaseInOrder(t *testing.T) {
	db := testutil.NewDB(t, nil)

	applied, err := dbmigrate.Run(context.Background(), db, load(t, base), dbmigrate.Options{})

	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 3 || applied[0] != 1 || applied[2] != 3 {
		t.Errorf("applied = %v, want [1 2 3]", applied)
	}
	if count(t, db, "widgets") != 2 {
		t.Error("the seed migration did not run")
	}
	versions, _ := dbmigrate.Status(context.Background(), db, dbmigrate.Options{})
	if len(versions) != 3 {
		t.Errorf("Status = %v", versions)
	}
	// 3つ目の ALTER も効いている。
	var color string
	if err := db.Raw("SELECT color FROM widgets LIMIT 1").Scan(&color).Error; err != nil || color != "red" {
		t.Errorf("color = %q err=%v", color, err)
	}
}

// 2回目の起動では、何も適用しない(適用済みは、飛ばす)。新しいファイルだけが、足される。
func TestRerunAppliesOnlyWhatIsNew(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	if _, err := dbmigrate.Run(ctx, db, load(t, base), dbmigrate.Options{}); err != nil {
		t.Fatal(err)
	}

	again, err := dbmigrate.Run(ctx, db, load(t, base), dbmigrate.Options{})
	if err != nil || len(again) != 0 {
		t.Fatalf("a second run applied %v (err=%v), want nothing", again, err)
	}

	withNew := map[string]string{"000004_more.sql": "INSERT INTO widgets (name) VALUES ('third');"}
	for name, sql := range base {
		withNew[name] = sql
	}
	applied, err := dbmigrate.Run(ctx, db, load(t, withNew), dbmigrate.Options{})
	if err != nil || len(applied) != 1 || applied[0] != 4 {
		t.Errorf("applied = %v (err=%v), want only version 4", applied, err)
	}
	if count(t, db, "widgets") != 3 {
		t.Errorf("rows = %d, want 3 (the seed must not run twice)", count(t, db, "widgets"))
	}
}

// 適用したファイルを書き換えたら、何も適用せずに、エラーにする。
func TestModifyingAnAppliedMigrationIsRejected(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	if _, err := dbmigrate.Run(ctx, db, load(t, base), dbmigrate.Options{}); err != nil {
		t.Fatal(err)
	}

	tampered := map[string]string{"000004_new.sql": "INSERT INTO widgets (name) VALUES ('new');"}
	for name, sql := range base {
		tampered[name] = sql
	}
	tampered["000002_seed.sql"] = "INSERT INTO widgets (name) VALUES ('edited');"

	applied, err := dbmigrate.Run(ctx, db, load(t, tampered), dbmigrate.Options{})

	if err == nil || !strings.Contains(err.Error(), "modified after it was applied") {
		t.Fatalf("err = %v, want a checksum error", err)
	}
	if len(applied) != 0 || count(t, db, "widgets") != 2 {
		t.Errorf("nothing may be applied when a migration was modified: applied=%v rows=%d", applied, count(t, db, "widgets"))
	}
}

// 1つのファイルは、1つのトランザクション。途中で失敗したら、そのファイルの変更は全部取り消され、記録も残らない。
func TestAFailingMigrationIsRolledBackAndNotRecorded(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	broken := map[string]string{
		"000001_baseline.sql": "CREATE TABLE widgets (id serial PRIMARY KEY, name text NOT NULL);",
		// 1文目は成功し、2文目で失敗する。1文目の INSERT も、取り消される。
		"000002_broken.sql": "INSERT INTO widgets (name) VALUES ('half');\nINSERT INTO no_such_table VALUES (1);",
	}

	applied, err := dbmigrate.Run(ctx, db, load(t, broken), dbmigrate.Options{})

	if err == nil || !strings.Contains(err.Error(), "000002_broken") {
		t.Fatalf("err = %v, want the failing migration to be named", err)
	}
	if len(applied) != 1 || applied[0] != 1 {
		t.Errorf("applied = %v, want only the baseline", applied)
	}
	if count(t, db, "widgets") != 0 {
		t.Error("the failed migration's first statement was not rolled back")
	}
	versions, _ := dbmigrate.Status(ctx, db, dbmigrate.Options{})
	if len(versions) != 1 {
		t.Errorf("recorded versions = %v, want only [1]", versions)
	}

	// 直したら(同じ番号のファイルを、まだ適用されていないので、直してよい)、続きから適用される。
	broken["000002_broken.sql"] = "INSERT INTO widgets (name) VALUES ('fixed');"
	applied, err = dbmigrate.Run(ctx, db, load(t, broken), dbmigrate.Options{})
	if err != nil || len(applied) != 1 || applied[0] != 2 || count(t, db, "widgets") != 1 {
		t.Errorf("after the fix: applied=%v err=%v rows=%d", applied, err, count(t, db, "widgets"))
	}
}

// 以前(AutoMigrate)に作られた DB: ベースラインは、実行せずに、適用済みと記録する。以降のファイルは、普通に適用する。
func TestAnExistingDatabaseSkipsTheBaseline(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	// 「以前の AutoMigrate で、すでにテーブルがある」状態を作る(中身がある)。
	if err := db.Exec("CREATE TABLE widgets (id serial PRIMARY KEY, name text NOT NULL); INSERT INTO widgets (name) VALUES ('existing');").Error; err != nil {
		t.Fatal(err)
	}

	applied, err := dbmigrate.Run(ctx, db, load(t, base), dbmigrate.Options{BaselineVersion: 1, SentinelTable: "widgets"})

	// ベースライン(CREATE TABLE)を実行していたら、「すでにある」で失敗している。
	if err != nil {
		t.Fatalf("the baseline must not be executed against an existing database: %v", err)
	}
	if len(applied) != 2 || applied[0] != 2 || applied[1] != 3 {
		t.Errorf("applied = %v, want [2 3]", applied)
	}
	versions, _ := dbmigrate.Status(ctx, db, dbmigrate.Options{})
	if len(versions) != 3 || versions[0] != 1 {
		t.Errorf("the baseline must be recorded as applied: %v", versions)
	}
	if count(t, db, "widgets") != 3 { // 既存の1行 + 2つ目の seed の2行
		t.Errorf("rows = %d, want 3", count(t, db, "widgets"))
	}
}

// 新しい(空の)DB では、SentinelTable が無いので、ベースラインも普通に実行される。
func TestANewDatabaseRunsTheBaselineEvenWithBaselineOptions(t *testing.T) {
	db := testutil.NewDB(t, nil)

	applied, err := dbmigrate.Run(context.Background(), db, load(t, base), dbmigrate.Options{BaselineVersion: 1, SentinelTable: "widgets"})

	if err != nil || len(applied) != 3 {
		t.Errorf("applied=%v err=%v, want all three to run", applied, err)
	}
}

// 2回目以降の起動で、ベースラインの判定が、また働いてはいけない(記録があるので、判定しない)。
func TestBaselineDetectionOnlyHappensOnce(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	options := dbmigrate.Options{BaselineVersion: 1, SentinelTable: "widgets"}
	if _, err := dbmigrate.Run(ctx, db, load(t, base), options); err != nil {
		t.Fatal(err)
	}

	applied, err := dbmigrate.Run(ctx, db, load(t, base), options)

	if err != nil || len(applied) != 0 {
		t.Errorf("applied=%v err=%v, want nothing", applied, err)
	}
	versions, _ := dbmigrate.Status(ctx, db, dbmigrate.Options{})
	if len(versions) != 3 {
		t.Errorf("versions = %v", versions)
	}
}

// 複数の Pod が同時に起動しても、各ファイルは、1回だけ適用される(アドバイザリロック)。
func TestConcurrentStartupsApplyEachMigrationOnce(t *testing.T) {
	db := testutil.NewDB(t, nil)
	migrations := load(t, base)

	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := dbmigrate.Run(context.Background(), db, migrations, dbmigrate.Options{})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("a concurrent run failed: %v", err)
		}
	}
	if count(t, db, "widgets") != 2 {
		t.Errorf("rows = %d, want 2 (the seed must run exactly once)", count(t, db, "widgets"))
	}
}

// DB のほうが新しい(古いバージョンのアプリに戻したとき)は、エラーにしない。
func TestAnOlderAppAgainstANewerDatabaseStillStarts(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	newer := map[string]string{"000004_more.sql": "INSERT INTO widgets (name) VALUES ('third');"}
	for name, sql := range base {
		newer[name] = sql
	}
	if _, err := dbmigrate.Run(ctx, db, load(t, newer), dbmigrate.Options{}); err != nil {
		t.Fatal(err)
	}

	applied, err := dbmigrate.Run(ctx, db, load(t, base), dbmigrate.Options{}) // 古いアプリは、1〜3 しか知らない

	if err != nil || len(applied) != 0 {
		t.Errorf("applied=%v err=%v; rolling the app back must not break startup", applied, err)
	}
}

func TestStatusOfADatabaseWithoutMigrations(t *testing.T) {
	db := testutil.NewDB(t, nil)
	versions, err := dbmigrate.Status(context.Background(), db, dbmigrate.Options{})
	if err != nil || len(versions) != 0 {
		t.Errorf("versions=%v err=%v, want empty", versions, err)
	}
}

// 1つのデータベースを、複数のサービスが共有する(開発・staging)ときも、サービスごとの記録のテーブルを分ければ、
// 互いの「番号 1」(ベースライン)がぶつからない。実際に、共有の記録で、起動に失敗したことがあった。
func TestServicesSharingADatabaseKeepSeparateRecords(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	orcan := load(t, map[string]string{"000001_baseline.sql": "CREATE TABLE orcan_things (id int);"})
	payment := load(t, map[string]string{"000001_baseline.sql": "CREATE TABLE payment_things (id int);"})

	if _, err := dbmigrate.Run(ctx, db, orcan, dbmigrate.Options{Table: "orcan_schema_migrations"}); err != nil {
		t.Fatal(err)
	}
	// 同じ番号 1 でも、中身(チェックサム)が違う。記録を共有していたら、ここで失敗する。
	if _, err := dbmigrate.Run(ctx, db, payment, dbmigrate.Options{Table: "payment_schema_migrations"}); err != nil {
		t.Fatalf("a second service sharing the database must not collide with the first: %v", err)
	}
	// 再起動しても、どちらも失敗しない。
	for _, run := range []struct {
		migrations []dbmigrate.Migration
		table      string
	}{{orcan, "orcan_schema_migrations"}, {payment, "payment_schema_migrations"}} {
		if applied, err := dbmigrate.Run(ctx, db, run.migrations, dbmigrate.Options{Table: run.table}); err != nil || len(applied) != 0 {
			t.Errorf("%s: applied=%v err=%v", run.table, applied, err)
		}
	}
}

func TestInvalidTableNamesAreRejected(t *testing.T) {
	db := testutil.NewDB(t, nil)
	for _, bad := range []string{"x; DROP TABLE users", "has space", "1starts_with_digit", "quote\"d", "a-b"} {
		if _, err := dbmigrate.Run(context.Background(), db, load(t, base), dbmigrate.Options{Table: bad}); err == nil {
			t.Errorf("table name %q was accepted", bad)
		}
	}
}
