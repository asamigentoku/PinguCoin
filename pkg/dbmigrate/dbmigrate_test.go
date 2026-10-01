package dbmigrate

import (
	"strings"
	"testing"
	"testing/fstest"
)

func files(entries map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, content := range entries {
		fsys["migrations/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func TestLoadSortsByVersion(t *testing.T) {
	migrations, err := Load(files(map[string]string{
		"000003_add_index.sql": "CREATE INDEX i ON t (a);",
		"000001_baseline.sql":  "CREATE TABLE t (a int);",
		"000002_seed_data.sql": "INSERT INTO t VALUES (1);",
		"README.md":            "not a migration",
	}), "migrations")
	if err != nil {
		t.Fatal(err)
	}

	if len(migrations) != 3 || migrations[0].Name != "baseline" || migrations[1].Name != "seed_data" || migrations[2].Name != "add_index" {
		t.Fatalf("unexpected order: %+v", migrations)
	}
	for i, migration := range migrations {
		if migration.Version != int64(i+1) || migration.SQL == "" || len(migration.Checksum) != 64 {
			t.Errorf("unexpected migration: %+v", migration)
		}
	}
}

func TestLoadRejectsMistakes(t *testing.T) {
	tests := []struct {
		name    string
		entries map[string]string
		want    string
	}{
		{"a badly named file", map[string]string{"1_baseline.sql": "SELECT 1;"}, "must be named NNNNNN_description.sql"},
		{"a name with spaces", map[string]string{"000001_my file.sql": "SELECT 1;"}, "must be named"},
		{"a duplicated version", map[string]string{"000001_a.sql": "SELECT 1;", "000001_b.sql": "SELECT 2;"}, "used by two files"},
		{"a gap in the numbers", map[string]string{"000001_a.sql": "SELECT 1;", "000003_c.sql": "SELECT 3;"}, "without gaps"},
		{"not starting at 1", map[string]string{"000002_a.sql": "SELECT 1;"}, "without gaps"},
		{"an empty file", map[string]string{"000001_a.sql": "   \n"}, "no SQL statements"},
		{"a file with only comments", map[string]string{"000001_a.sql": "-- TODO\n-- later\n"}, "no SQL statements"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(files(tt.entries), "migrations")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestLoadFailsWhenTheDirectoryIsMissing(t *testing.T) {
	if _, err := Load(fstest.MapFS{}, "nope"); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

// Windows の改行(CRLF)と LF で、チェックサムが変わらない(OS による違いで、起動に失敗しない)。
func TestChecksumIgnoresLineEndings(t *testing.T) {
	lf, err := Load(files(map[string]string{"000001_a.sql": "CREATE TABLE t (\n  a int\n);\n"}), "migrations")
	if err != nil {
		t.Fatal(err)
	}
	crlf, err := Load(files(map[string]string{"000001_a.sql": "CREATE TABLE t (\r\n  a int\r\n);\r\n"}), "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if lf[0].Checksum != crlf[0].Checksum {
		t.Error("CRLF and LF must produce the same checksum")
	}
}

func TestChecksumChangesWhenTheSQLChanges(t *testing.T) {
	a, _ := Load(files(map[string]string{"000001_a.sql": "CREATE TABLE t (a int);"}), "migrations")
	b, _ := Load(files(map[string]string{"000001_a.sql": "CREATE TABLE t (a bigint);"}), "migrations")
	if a[0].Checksum == b[0].Checksum {
		t.Error("a modified migration must have a different checksum")
	}
}
