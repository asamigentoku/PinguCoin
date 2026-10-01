package database_test

import (
	"sort"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/testutil"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
)

// このテストは、SQL のマイグレーション(正)と、Go のモデルの定義が、食い違っていないことを確かめる。
//
// モデルを変えたのに、マイグレーションを足し忘れると、コードは新しい列を使うのに、本番の DB には無い、という事故になる
// (以前の AutoMigrate は、起動時に自動で差分を当てていたので、起きなかった)。このテストが失敗したら、
// モデルの変更に対応する、新しいマイグレーションのファイルを足す(docs/VERSIONING.md)。
// 実際の Postgres(TEST_DATABASE_URL)が必要。

// modelSchema は、モデルから gorm が作る、スキーマ(以前の AutoMigrate が作っていたもの)。
func modelSchema(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Payment{},
		&model.Refund{},
		&model.PointAccount{},
		&model.PointTransaction{},
	)
}

type column struct {
	Table, Column, DataType, Nullable, Default string
}

func columns(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var rows []column
	err := db.Raw(`
		SELECT table_name AS "table", column_name AS "column", data_type AS data_type, is_nullable AS nullable, COALESCE(column_default, '') AS "default"
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name NOT LIKE '%schema_migrations'
		ORDER BY table_name, column_name`).Scan(&rows).Error
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Table+"."+r.Column+" "+r.DataType+" nullable="+r.Nullable+" default="+r.Default)
	}
	return out
}

func names(t *testing.T, db *gorm.DB, query string) []string {
	t.Helper()
	var out []string
	if err := db.Raw(query).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// SQL のマイグレーションにだけあって、モデル(gorm のタグ)では表せないもの。ずれとは扱わない。
var sqlOnly = map[string]string{}

// ignored は、比べない対象: マイグレーションの記録のテーブル(schema_migrations)と、SQL だけにあるもの。
func ignored(name string) bool {
	if table, _, _ := strings.Cut(name, "."); strings.HasSuffix(table, "schema_migrations") {
		return true
	}
	_, onlySQL := sqlOnly[name]
	return onlySQL
}

func diff(t *testing.T, what string, fromMigrations, fromModels []string) {
	t.Helper()
	inModels := map[string]bool{}
	for _, s := range fromModels {
		inModels[s] = true
	}
	inMigrations := map[string]bool{}
	for _, s := range fromMigrations {
		if ignored(s) {
			continue
		}
		inMigrations[s] = true
		if !inModels[s] {
			t.Errorf("%s: only in the migrations (not in the models): %s", what, s)
		}
	}
	for _, s := range fromModels {
		if !inMigrations[s] {
			t.Errorf("%s: only in the models (a migration is missing): %s", what, s)
		}
	}
}

func TestMigrationsMatchTheModels(t *testing.T) {
	migrated := testutil.NewDB(t, database.Migrate)
	modelled := testutil.NewDB(t, modelSchema)

	diff(t, "columns", columns(t, migrated), columns(t, modelled))
	diff(t, "tables", names(t, migrated, `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_name NOT LIKE '%schema_migrations'`),
		names(t, modelled, `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema()`))
	// 制約(主キー・外部キー・UNIQUE)の名前。
	diff(t, "constraints", names(t, migrated, `SELECT conrelid::regclass::text || '.' || conname FROM pg_constraint WHERE connamespace = current_schema()::regnamespace`),
		names(t, modelled, `SELECT conrelid::regclass::text || '.' || conname FROM pg_constraint WHERE connamespace = current_schema()::regnamespace`))
	// インデックスの名前。
	diff(t, "indexes", names(t, migrated, `SELECT tablename || '.' || indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename NOT LIKE '%schema_migrations'`),
		names(t, modelled, `SELECT tablename || '.' || indexname FROM pg_indexes WHERE schemaname = current_schema()`))
}
