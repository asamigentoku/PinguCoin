// Package dbmigrate は、DB のスキーマを、バージョン付きの SQL ファイルで管理する(マイグレーション)。
//
// 以前の AutoMigrate(gorm がモデルから、起動のたびに差分を当てる)をやめて、次のようにする。
//
//   - 変更は、番号付きの SQL ファイルとして、コードと一緒にコミットする(services/<service>/migrations/000002_xxx.sql)。
//     「どのバージョンのアプリが、どのスキーマを前提にしているか」「いつ、何を変えたか」が、Git の履歴に残る。
//   - どこまで適用したかは、DB の schema_migrations テーブルに記録する。起動時に、未適用のものを、番号順に1つずつ適用する。
//   - 1つのファイルは、1つのトランザクションで適用する。途中で失敗したら、そのファイルの変更は全部取り消され、
//     記録も残らない(次の起動で、最初からやり直す)。
//   - 適用したファイルは、書き換えない。書き換えると、チェックサムが合わず、起動に失敗する(適用済みの DB と、
//     新しい DB で、スキーマが食い違うのを防ぐ)。変更は、新しいファイルで足す。
//   - 前にしか進まない(down のファイルは無い)。間違えたときは、直す SQL を、新しいファイルで足す(fix forward)。
//   - 複数の Pod が同時に起動しても、アドバイザリロック(pkg/migrate)で、1つずつ実行する。
//
// 既存のデータベース(AutoMigrate で作られた)の扱い:
//
//	番号 1 のファイルは、導入した時点のスキーマ全体(ベースライン)。すでにそのスキーマがある DB では、実行せずに、
//	「適用済み」と記録する(SentinelTable というテーブルがあれば、既存の DB と判断する)。新しい DB では、普通に実行する。
//
// 記録のテーブルは、サービスごとに別にする(Options.Table。例: orcan_schema_migrations)。
// 開発・staging では、3つのサービスが、同じデータベースを共有することがある。記録を共有すると、各サービスの
// 「番号 1」(ベースライン)が、ぶつかってしまう。本番のように、データベースが別々でも、同じ決まりにしておく。
//
// ファイル名は NNNNNN_説明.sql(NNNNNN は、1 から始まる連番)。
package dbmigrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/migrate"
)

// Migration は、1つのマイグレーション(SQL ファイル)。
type Migration struct {
	Version  int64  // ファイル名の番号
	Name     string // ファイル名の説明の部分
	SQL      string
	Checksum string // SQL の SHA-256(改行コードの違いは無視する)
}

var fileName = regexp.MustCompile(`^(\d{6})_([A-Za-z0-9_]+)\.sql$`)

// Load は、fsys の dir にある NNNNNN_xxx.sql を読み込んで、番号順に返す。
// 番号の重複、1 から始まらない・途切れた連番、空のファイル、命名規則に合わないファイルは、エラーにする
// (ファイルの置き忘れ・番号の取り合いに、早く気づくため)。
func Load(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := fileName.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("migration file %q must be named NNNNNN_description.sql (six digits, then letters/digits/underscores)", entry.Name())
		}
		version, _ := strconv.ParseInt(match[1], 10, 64)
		raw, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		sql := normalizeNewlines(string(raw))
		if strings.TrimSpace(stripComments(sql)) == "" {
			return nil, fmt.Errorf("migration %s has no SQL statements", entry.Name())
		}
		migrations = append(migrations, Migration{Version: version, Name: match[2], SQL: sql, Checksum: checksum(sql)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for i, migration := range migrations {
		want := int64(i + 1)
		switch {
		case i > 0 && migration.Version == migrations[i-1].Version:
			return nil, fmt.Errorf("migration version %06d is used by two files (%s and %s)", migration.Version, migrations[i-1].Name, migration.Name)
		case migration.Version != want:
			return nil, fmt.Errorf("migration versions must be 1, 2, 3, ... without gaps: expected %06d but found %06d_%s", want, migration.Version, migration.Name)
		}
	}
	return migrations, nil
}

// Options は、Run の設定。
type Options struct {
	// BaselineVersion は、「導入した時点のスキーマ全体」のファイルの番号(通常 1)。0 なら、ベースラインの扱いをしない。
	BaselineVersion int64
	// SentinelTable は、「すでにベースラインのスキーマがある」ことを判断するためのテーブル名(例: products)。
	// schema_migrations が空で、このテーブルがあれば、既存の DB と判断して、BaselineVersion までを、実行せずに記録する。
	SentinelTable string
	// Table は、適用の記録を持つテーブルの名前。空なら schema_migrations。
	// 1つのデータベースを、複数のサービスで共有するときは、サービスごとに別の名前にする(英数字と _ だけ)。
	Table string
	// Logger は、適用の記録を出す先。nil なら、出さない。
	Logger *slog.Logger
}

// DefaultTable は、Options.Table を指定しなかったときの、記録のテーブル名。
const DefaultTable = "schema_migrations"

var tableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// table は、記録のテーブル名(SQL に埋め込むので、安全な名前だけ受け付ける)。
func (o Options) table() (string, error) {
	name := o.Table
	if name == "" {
		name = DefaultTable
	}
	if !tableName.MatchString(name) {
		return "", fmt.Errorf("invalid migrations table name %q (letters, digits and underscores only)", name)
	}
	return name, nil
}

func createTableSQL(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
	version    BIGINT      PRIMARY KEY,
	name       TEXT        NOT NULL,
	checksum   TEXT        NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`
}

// Run は、未適用のマイグレーションを、番号順に適用する。適用した番号を返す。
// 適用済みのファイルが書き換えられていたら、何も適用せずに、エラーにする。
func Run(ctx context.Context, db *gorm.DB, migrations []Migration, options Options) ([]int64, error) {
	var applied []int64
	err := migrate.WithLock(ctx, db, func(conn *gorm.DB) error {
		var err error
		applied, err = run(ctx, conn, migrations, options)
		return err
	})
	return applied, err
}

func run(ctx context.Context, conn *gorm.DB, migrations []Migration, options Options) ([]int64, error) {
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	table, err := options.table()
	if err != nil {
		return nil, err
	}

	if err := conn.WithContext(ctx).Exec(createTableSQL(table)).Error; err != nil {
		return nil, fmt.Errorf("create %s: %w", table, err)
	}
	recorded, err := recordedChecksums(ctx, conn, table)
	if err != nil {
		return nil, err
	}

	// 以前の AutoMigrate で作られた DB: ベースラインは、実行せずに、適用済みと記録する。
	if len(recorded) == 0 && options.BaselineVersion > 0 && options.SentinelTable != "" {
		exists, err := tableExists(ctx, conn, options.SentinelTable)
		if err != nil {
			return nil, err
		}
		if exists {
			for _, migration := range migrations {
				if migration.Version > options.BaselineVersion {
					break
				}
				if err := record(ctx, conn, table, migration); err != nil {
					return nil, err
				}
				recorded[migration.Version] = migration.Checksum
				logger.Info("existing database: marked the baseline migration as applied (not executed)",
					slog.Int64("migration_version", migration.Version), slog.String("migration_name", migration.Name))
			}
		}
	}

	// 適用済みのファイルが、書き換えられていないか。1つでも違えば、何も適用しない。
	known := map[int64]bool{}
	for _, migration := range migrations {
		known[migration.Version] = true
		if sum, done := recorded[migration.Version]; done && sum != migration.Checksum {
			return nil, fmt.Errorf("migration %06d_%s was modified after it was applied (checksum mismatch); "+
				"never edit an applied migration — add a new one instead", migration.Version, migration.Name)
		}
	}
	// DB のほうが新しい(古いバージョンのアプリに、戻したとき)。普通のことなので、エラーにはしない。
	for version := range recorded {
		if !known[version] {
			logger.Warn("the database has migrations this version of the app does not know (running an older version?)",
				slog.Int64("migration_version", version))
		}
	}

	var applied []int64
	for _, migration := range migrations {
		if _, done := recorded[migration.Version]; done {
			continue
		}
		begin := time.Now()
		err := conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(migration.SQL).Error; err != nil {
				return err
			}
			return record(ctx, tx, table, migration)
		})
		if err != nil {
			return applied, fmt.Errorf("apply migration %06d_%s: %w", migration.Version, migration.Name, err)
		}
		applied = append(applied, migration.Version)
		logger.Info("applied database migration",
			slog.Int64("migration_version", migration.Version), slog.String("migration_name", migration.Name),
			slog.Int64("duration", time.Since(begin).Nanoseconds()))
	}
	return applied, nil
}

// Status は、適用済みのバージョンの一覧(古い順)。記録のテーブルが無ければ、空。
func Status(ctx context.Context, db *gorm.DB, options Options) ([]int64, error) {
	table, err := options.table()
	if err != nil {
		return nil, err
	}
	exists, err := tableExists(ctx, db, table)
	if err != nil || !exists {
		return nil, err
	}
	var versions []int64
	err = db.WithContext(ctx).Raw("SELECT version FROM " + table + " ORDER BY version").Scan(&versions).Error
	return versions, err
}

func recordedChecksums(ctx context.Context, conn *gorm.DB, table string) (map[int64]string, error) {
	var rows []struct {
		Version  int64
		Checksum string
	}
	if err := conn.WithContext(ctx).Raw("SELECT version, checksum FROM " + table).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read %s: %w", table, err)
	}
	recorded := make(map[int64]string, len(rows))
	for _, row := range rows {
		recorded[row.Version] = row.Checksum
	}
	return recorded, nil
}

func record(ctx context.Context, conn *gorm.DB, table string, migration Migration) error {
	return conn.WithContext(ctx).Exec(
		"INSERT INTO "+table+" (version, name, checksum) VALUES (?, ?, ?)",
		migration.Version, migration.Name, migration.Checksum,
	).Error
}

func tableExists(ctx context.Context, conn *gorm.DB, table string) (bool, error) {
	var exists bool
	// to_regclass は、現在の search_path で、テーブルを探す。
	if err := conn.WithContext(ctx).Raw("SELECT to_regclass(?) IS NOT NULL", table).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check table %q: %w", table, err)
	}
	return exists, nil
}

// normalizeNewlines は、Windows の改行(CRLF)を LF にそろえる(OS による改行の違いで、チェックサムが変わらないように)。
func normalizeNewlines(sql string) string {
	return strings.ReplaceAll(sql, "\r\n", "\n")
}

func checksum(sql string) string {
	sum := sha256.Sum256([]byte(sql))
	return hex.EncodeToString(sum[:])
}

// stripComments は、行頭の -- コメントを除く(コメントしかないファイルを、空として検出するため)。
func stripComments(sql string) string {
	var kept []string
	for _, line := range strings.Split(sql, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
