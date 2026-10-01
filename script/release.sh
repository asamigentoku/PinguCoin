#!/usr/bin/env bash
# リリースのタグ(vMAJOR.MINOR.PATCH)を付ける。push すると、release.yml が、テストのあとに GitHub Release を作る。
#
#   bash script/release.sh v1.2.3            # タグを付ける(push はしない。次にやることを表示する)
#   bash script/release.sh v1.2.3 --push     # タグを付けて、origin に push する(= リリースを作る)
#   bash script/release.sh v1.2.3 --dry-run  # 確認だけ(何も変えない)
#   bash script/release.sh next              # 直近のタグから、次のバージョンの候補を表示する
#
# 確認すること(どれかが満たされないと、タグを付けない):
#   - バージョンが、セマンティックバージョニングの形式(v1.2.3、プレリリースは v1.2.3-rc.1)
#   - main ブランチにいて、作業ツリーがきれいで、origin/main と同じ(まだ push していない変更を、リリースしない)
#   - 同じタグが、まだ無い
# バージョンの決め方は docs/VERSIONING.md を参照。
set -euo pipefail

usage() { sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 1; }
[ $# -ge 1 ] || usage

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

SEMVER='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'

if [ "$1" = "next" ]; then
  latest="$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)"
  if [ -z "$latest" ]; then echo "まだリリースがありません。最初は v0.1.0 か v1.0.0 から。"; exit 0; fi
  IFS=. read -r major minor patch <<<"${latest#v}"
  echo "直近のリリース: $latest"
  echo "  バグ修正だけ         : v$major.$minor.$((patch + 1))"
  echo "  後方互換の機能の追加 : v$major.$((minor + 1)).0"
  echo "  互換性を壊す変更     : v$((major + 1)).0.0"
  exit 0
fi

TAG="$1"
PUSH=false
DRY_RUN=false
shift
for arg in "$@"; do
  case "$arg" in
    --push) PUSH=true ;;
    --dry-run) DRY_RUN=true ;;
    *) echo "Unknown option: $arg" >&2; usage ;;
  esac
done

if ! [[ "$TAG" =~ $SEMVER ]]; then
  echo "バージョン '$TAG' は、vMAJOR.MINOR.PATCH の形式ではありません(例: v1.2.3、v1.2.3-rc.1)" >&2
  exit 1
fi
if [ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then
  echo "main ブランチでタグを付けてください(いま: $(git rev-parse --abbrev-ref HEAD))" >&2
  exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "コミットしていない変更があります。きれいな状態でタグを付けてください" >&2
  exit 1
fi
git fetch origin main --tags --quiet
if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
  echo "ローカルの main が、origin/main と違います(pull / push してください)" >&2
  exit 1
fi
if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "タグ $TAG は、すでにあります" >&2
  exit 1
fi

echo "リリースするコミット: $(git log -1 --format='%h %s')"
previous="$(git describe --tags --abbrev=0 2>/dev/null || true)"
[ -n "$previous" ] && echo "前のリリース: $previous(以降の変更: $(git rev-list --count "$previous"..HEAD) コミット)"

if [ "$DRY_RUN" = true ]; then
  echo "(--dry-run: タグは付けません。問題ありません)"
  exit 0
fi

git tag -a "$TAG" -m "Release $TAG"
echo "タグ $TAG を付けました。"
if [ "$PUSH" = true ]; then
  git push origin "$TAG"
  echo "push しました。GitHub Actions の Release が、テストのあとに、GitHub Release を作ります。"
else
  echo "リリースを作るには:  git push origin $TAG"
  echo "取り消すには(push 前):  git tag -d $TAG"
fi
