package version

import (
	"strings"
	"testing"
)

// テスト中だけ、埋め込まれる変数を差し替える。
func withLdflags(t *testing.T, version, commit, buildTime string) {
	t.Helper()
	previousVersion, previousCommit, previousBuildTime := Version, Commit, BuildTime
	Version, Commit, BuildTime = version, commit, buildTime
	t.Cleanup(func() { Version, Commit, BuildTime = previousVersion, previousCommit, previousBuildTime })
}

func TestEmbeddedValuesAreUsed(t *testing.T) {
	withLdflags(t, "v1.2.3", "1a2b3c4d5e6f7a8b", "2026-10-01T09:00:00Z")

	info := Get()

	if info.Version != "v1.2.3" || info.Commit != "1a2b3c4d5e6f7a8b" || info.BuildTime != "2026-10-01T09:00:00Z" {
		t.Errorf("unexpected info: %+v", info)
	}
	if info.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
}

// 埋め込まれていないとき(go run / go test)は、バージョンは "dev"。空にはならない。
func TestDefaultsWhenNothingIsEmbedded(t *testing.T) {
	withLdflags(t, "", "", "")

	info := Get()

	if info.Version != "dev" {
		t.Errorf("Version = %q, want dev", info.Version)
	}
	if info.Commit == "" || info.BuildTime == "" {
		t.Errorf("Commit and BuildTime must never be empty: %+v", info)
	}
}

func TestShortCommit(t *testing.T) {
	if got := (Info{Commit: "1a2b3c4d5e6f"}).ShortCommit(); got != "1a2b3c4" {
		t.Errorf("ShortCommit = %q", got)
	}
	if got := (Info{Commit: "abc"}).ShortCommit(); got != "abc" {
		t.Errorf("a short commit should stay as it is: %q", got)
	}
}

func TestString(t *testing.T) {
	got := Info{Version: "v1.2.3", Commit: "1a2b3c4d5e6f", BuildTime: "2026-10-01T09:00:00Z", GoVersion: "go1.27.1"}.String()
	want := "v1.2.3 (commit 1a2b3c4, built 2026-10-01T09:00:00Z, 1.27.1)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if strings.Contains(got, "go1.27.1") {
		t.Error("the go prefix should be trimmed")
	}
}
