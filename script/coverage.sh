#!/usr/bin/env bash
# テストのカバレッジ(コードのうち、テストで実行された割合)を測る。CI(_go-test.yml)と、ローカル(make cover)で同じものを使う。
#
#   bash script/coverage.sh                    # 測って、パッケージごとの割合と、全体の割合を表示する
#   MIN_COVERAGE=60 bash script/coverage.sh    # 全体の割合が 60% 未満なら、失敗する
#   TEST_DATABASE_URL=... bash script/coverage.sh   # DB を使う統合テストも含める(make cover-db)
#   COVERAGE_REUSE=1 bash script/coverage.sh   # テストは実行せず、既にある coverage.raw.out の集計だけをやり直す
#
# 出力(リポジトリのルート。Git の管理対象外):
#   coverage.out  ... カバレッジのデータ(生成コード・エントリーポイント・テスト用部品を除いたもの)
#   coverage.html ... 行ごとに、テストで通った所(緑)と通っていない所(赤)が分かるレポート
#
# 数え方:
#   - -coverpkg で、サービスと pkg/ の全体を対象にする(あるパッケージのテストが、別のパッケージのコードを通ったら、それも数える)
#   - 自動生成のコード(*.pb.go、graph/generated.go など)、cmd/(main)、pkg/testutil は、テストする意味が薄いので除く
#   - DB を使う統合テストは、TEST_DATABASE_URL が無いとスキップされる。その分は、カバーされていない扱いになる
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

RAW="coverage.raw.out"
OUT="${COVERAGE_OUT:-coverage.out}"
HTML="${COVERAGE_HTML:-coverage.html}"
MIN_COVERAGE="${MIN_COVERAGE:-0}"

if [ -z "${TEST_DATABASE_URL:-}" ]; then
  echo "note: TEST_DATABASE_URL is not set; the database integration tests are skipped (not counted as covered)." >&2
fi

if [ "${COVERAGE_REUSE:-}" = "1" ]; then
  [ -f "$RAW" ] || { echo "COVERAGE_REUSE=1 needs an existing $RAW" >&2; exit 1; }
else
  go test ./... -count=1 -covermode=atomic -coverpkg=./services/...,./pkg/... -coverprofile="$RAW"
fi

# 自動生成のコード・エントリーポイント・テスト用部品を除く(先頭の mode: の行は残る)。
grep -vE '\.pb\.go:|/graph/generated\.go:|/graph/model/models_gen\.go:|/cmd/|/testutil/' "$RAW" > "$OUT"
[ "${COVERAGE_REUSE:-}" = "1" ] || rm -f "$RAW"

go tool cover -html="$OUT" -o "$HTML"

# パッケージごとの割合(文の数で数える)。プロファイルの行: <ファイル>:<開始>,<終了> <文の数> <実行回数>
echo
echo "== coverage by package =="
awk -F'[: ]' 'NR > 1 {
    path = $1; sub(/\/[^\/]+$/, "", path)
    stmts[path] += $(NF-1)
    if ($NF > 0) covered[path] += $(NF-1)
  }
  END { for (p in stmts) printf "%6.1f%%  %s\n", 100 * covered[p] / stmts[p], p }' "$OUT" \
  | sed 's|github.com/asamigentoku/PinguCoin/||' | sort -k2

TOTAL="$(go tool cover -func="$OUT" | awk '/^total:/ { gsub("%", "", $3); print $3 }')"
echo
echo "== total: ${TOTAL}% (minimum: ${MIN_COVERAGE}%) =="

# GitHub Actions のジョブの要約にも出す。
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo "## Go test coverage"
    echo
    echo "**Total: ${TOTAL}%** (minimum: ${MIN_COVERAGE}%)"
    echo
    echo "| package | coverage |"
    echo "| --- | --- |"
    awk -F'[: ]' 'NR > 1 {
        path = $1; sub(/\/[^\/]+$/, "", path)
        stmts[path] += $(NF-1)
        if ($NF > 0) covered[path] += $(NF-1)
      }
      END { for (p in stmts) printf "| %s | %.1f%% |\n", p, 100 * covered[p] / stmts[p] }' "$OUT" \
      | sed 's|github.com/asamigentoku/PinguCoin/||' | sort
  } >> "$GITHUB_STEP_SUMMARY"
fi

# 全体の割合が下限を下回ったら失敗する(カバレッジが、知らないうちに下がっていくのを防ぐ)。
if ! awk -v total="$TOTAL" -v min="$MIN_COVERAGE" 'BEGIN { exit !(total + 0 >= min + 0) }'; then
  echo "coverage ${TOTAL}% is below the minimum ${MIN_COVERAGE}%" >&2
  exit 1
fi
