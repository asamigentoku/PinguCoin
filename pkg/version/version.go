// Package version は、動いているプログラムが「どのバージョンか」を答える。
//
// バージョンは、ビルドのときに埋め込む(ldflags の -X)。Dockerfile が、次の値を渡す。
//
//	-X github.com/asamigentoku/PinguCoin/pkg/version.Version=v1.2.3
//	-X github.com/asamigentoku/PinguCoin/pkg/version.Commit=<git の SHA>
//	-X github.com/asamigentoku/PinguCoin/pkg/version.BuildTime=<ビルドした時刻(UTC)>
//
// 埋め込まれていない(`go run` や `go test` など)ときは、Go が実行ファイルに残すビルド情報(git の SHA と時刻)で補い、
// バージョンは "dev" とする。
//
// この情報は、ログの `version` 属性、`GET /version`(pingu-api)、起動時のログに使う。
// 「いま本番で動いているのは、どのバージョンか」「このログは、どのバージョンが出したか」を、すぐに答えられるようにするため。
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// ビルド時に -X で埋め込む値。空のときは、Go のビルド情報で補う。
var (
	Version   string
	Commit    string
	BuildTime string
)

// Info は、このプログラムのバージョン情報。
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// Get は、バージョン情報を返す。
func Get() Info {
	info := Info{Version: Version, Commit: Commit, BuildTime: BuildTime, GoVersion: runtime.Version()}

	if info.Commit == "" || info.BuildTime == "" {
		if build, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range build.Settings {
				switch {
				case setting.Key == "vcs.revision" && info.Commit == "":
					info.Commit = setting.Value
				case setting.Key == "vcs.time" && info.BuildTime == "":
					info.BuildTime = setting.Value
				}
			}
		}
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.BuildTime == "" {
		info.BuildTime = "unknown"
	}
	return info
}

// ShortCommit は、Commit の先頭 7 文字(git の短い SHA)。
func (i Info) ShortCommit() string {
	if len(i.Commit) > 7 {
		return i.Commit[:7]
	}
	return i.Commit
}

// String は、人が読む1行(例: "v1.2.3 (commit 1a2b3c4, built 2026-10-01T09:00:00Z, go1.27.1)")。
func (i Info) String() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s)", i.Version, i.ShortCommit(), i.BuildTime, strings.TrimPrefix(i.GoVersion, "go"))
}
