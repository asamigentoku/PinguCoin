# Dependabot(依存関係の自動更新)

依存ライブラリ・Docker イメージ・GitHub Actions・Terraform のプロバイダーに、新しいバージョンが出たら、**週に1回、すべてを1本のプルリクエストにまとめて**作ります。設定は [.github/dependabot.yml](../.github/dependabot.yml) です。

## 何を見張っているか

| 対象 | 場所 |
| --- | --- |
| Go のライブラリ | `go.mod`(ルートに1つ。3つのサービスと `pkg/` が使う) |
| npm のライブラリ | `apps/client-web`、`apps/admin-web` |
| Docker イメージ | `services/*/Dockerfile`(`golang`、`distroless`) |
| ローカル用のイメージ | `platform/docker/docker-compose.local.yml`(Postgres、Redis、Azurite) |
| GitHub Actions | `.github/workflows/*.yml`、`.github/actions/production/*/action.yml` |
| Terraform | `platform/terraform/envs/production`、`staging`(と、その子フォルダ)のプロバイダーとロックファイル |

上の全部が、**1つのグループ(`all-dependencies`)**に入り、1本の PR になります。

## 動き方

- **週に1回**(月曜 朝 9 時、日本時間)に確認して、更新があれば、**1本の PR**(ブランチ名は `dependabot/multi-ecosystem/all-dependencies-...`)を作ります。
- **メジャーバージョンの更新は、入れません**(`ignore`)。影響が大きく、1本にまとめると、1つのせいで、PR 全体が通らなくなるためです。メジャーを上げるときは、下の「メジャーを上げるとき」。
- PR のタイトルは `chore(deps): ...`、ラベルは `dependencies` です。
- PR にも、通常の PR と同じ **CI(`ci.yml`)が動きます**。ビルド・テスト・カバレッジ・Docker のビルド・Terraform の検証が通れば、安全に取り込めます。

### 1本にまとめることの、いい点と注意

| いい点 | 注意 |
| --- | --- |
| 通知・レビュー・マージが、週に1回で済む | 1つの更新のせいで CI が落ちると、PR 全体が止まる(原因の更新を外して、残りを先に入れる) |
| 更新が溜まらない | Go・npm・Docker・Terraform が混ざるので、変更の量が多い日は、レビューが大変 |
| | 古い依存を、まとめて上げるので、問題の切り分けが、少し難しい(PR の本文に、更新の一覧が出る) |

PR が大きすぎて扱いにくいときは、`dependabot.yml` の `multi-ecosystem-group` を、系統ごとのグループ(`go-and-npm` / `infra` など)に分けてください。

## 扱い方

1. CI が通っているのを確認する。
2. PR の本文の、更新の一覧と、変更履歴(リリースノート)を見る。
3. Terraform の更新が入っているときは、**手元で `terraform plan`** を確認する(下の表)。
4. マージする。落ちているときは、落ちた更新を特定して、Dependabot に `@dependabot ignore this dependency` と返す、または、手動で直す。

### 注意が要るもの

| 対象 | 注意 |
| --- | --- |
| `github.com/99designs/gqlgen` | **自動の更新から外しています**。上げたら、GraphQL の生成コードを作り直す必要があるためです(`cd services/pingu-api && go tool gqlgen generate`)。手動で上げて、生成コードも一緒にコミットしてください |
| `google.golang.org/protobuf` / `grpc` | 上げたら、生成コードも、最新のツールで作り直すとよいです(`make proto`)。CI の `proto` ジョブは、生成コードがコミット済みのものと同じかを確かめます |
| Dockerfile の `golang` | minor・major は、**自動の更新から外しています**。Go のバージョンは、`go.mod`・`mise.toml`・CI の `setup-go` と揃えて、手動で上げます(patch だけ、自動) |
| Terraform のプロバイダー | **本番の `terraform plan` は、Dependabot の PR では動きません**(Azure にログインする権限を、Dependabot に渡さないため)。マージ前に、手元で `terraform plan` を確認してください(`platform/terraform/envs/production/README.md`)。マージして `production` ブランチに入れると、通常どおり plan → 承認 → apply です |
| `next` / `react` | 画面の動作は、`npm run build` と、実際の画面で確認してください |

## メジャーを上げるとき

メジャーは、自動の PR に入りません。上げたいものがあるときは、手動で、1つずつ行います。

```bash
go get example.com/module@v2.0.0 && go mod tidy         # Go
cd apps/client-web && npm install next@latest           # npm
```

そのうえで、CI と、動作を確認してください。

## 変えたいとき

- **頻度・曜日**: `multi-ecosystem-groups` の `schedule`(`interval: daily` / `weekly` / `monthly`)。
- **系統ごとに分ける**: `multi-ecosystem-groups` にグループを足して、各 `updates` の `multi-ecosystem-group` を、振り分ける。
- **更新から外す**: `ignore`(例: 特定のライブラリ、または `update-types` でメジャーだけ)。
- **脆弱性の更新**: 上の設定とは別に、GitHub の **Dependabot security updates**(Settings > Code security)を有効にすると、脆弱性が見つかったときに、すぐに、個別の PR が作られます(週1回の確認を待ちません)。有効にしておくことをおすすめします。

## 補足: この設定の前提

`multi-ecosystem-groups`(系統をまたいで、1本の PR にまとめる機能)は、比較的新しい機能です。`dependabot.yml` が、GitHub の「Insights > Dependency graph > Dependabot」で、**エラーなしで読み込まれる**ことを、最初に確認してください。もし使えない環境のときは、系統ごとに、`groups`(1つの `updates` の中でまとめる)を使います。
