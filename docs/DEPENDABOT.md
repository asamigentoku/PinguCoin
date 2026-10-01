# Dependabot(依存関係の自動更新)

依存ライブラリ・Docker イメージ・GitHub Actions・Terraform のプロバイダーに、新しいバージョンが出たら、**プルリクエスト**を自動で作ります。設定は [.github/dependabot.yml](../.github/dependabot.yml) です。

## 何を見張っているか

| 対象 | 場所 | PR の単位 |
| --- | --- | --- |
| Go のライブラリ | `go.mod`(ルートに1つ。3つのサービスと `pkg/` が使う) | `grpc-protobuf` / `azure-sdk` / `gorm` / その他の minor・patch |
| npm のライブラリ | `apps/client-web`、`apps/admin-web` | `next-react` / `clerk`(client-web のみ)/ `tailwind` / その他の minor・patch |
| Docker イメージ | `services/*/Dockerfile`(`golang`、`distroless`) | イメージごと |
| ローカル用のイメージ | `platform/docker/docker-compose.local.yml`(Postgres、Redis、Azurite) | イメージごと |
| GitHub Actions | `.github/workflows/*.yml`、`.github/actions/production/*/action.yml` | minor・patch をまとめて |
| Terraform | `platform/terraform/envs/production`、`staging`(と、その子フォルダ)のプロバイダーとロックファイル | プロバイダーごと |

## 動き方

- **週に1回**(月曜 朝 9 時、日本時間)に確認して、更新があれば PR を作ります。
- 同じ系統のものは、**1つの PR にまとめます**(minor・patch だけ)。**メジャーバージョンの更新は、まとめず別の PR**にします(影響が大きいので、1つずつ確認するため)。
- 開く PR の数には、上限があります(Go と npm は 5、そのほかは 2〜3)。溜まったら、マージするか閉じるまで、新しい PR は増えません。
- PR のタイトルは `chore(deps): ...`、ラベルは `dependencies` と、対象(`go` など)です。
  - ラベルがリポジトリに無いと、Dependabot はそのラベルを付けられません。使う場合は、GitHub の Issues > Labels で作っておいてください(`dependencies` は、自動で作られます)。
- PR にも、通常の PR と同じ **CI(`ci.yml`)が動きます**。ビルド・テスト・カバレッジ・Docker のビルド・Terraform の検証が通れば、安全に取り込めます。

## 扱い方

1. CI が通っているのを確認する。
2. 変更履歴(リリースノート)を見る。Dependabot が PR の本文に載せます。
3. マージする。落ちているときは、メジャーバージョンの変更などで、コードの修正が要るのかを見る。

### 注意が要るもの

| 対象 | 注意 |
| --- | --- |
| `github.com/99designs/gqlgen` | **自動の更新から外しています**。上げたら、GraphQL の生成コードを作り直す必要があるためです(`cd services/pingu-api && go tool gqlgen generate`)。手動で上げて、生成コードも一緒にコミットしてください |
| `google.golang.org/protobuf` / `grpc` | `grpc-protobuf` グループで一緒に上がります。生成コードも最新のツールで作り直すとよいです(`make proto`)。CI の `proto` ジョブは、生成コードがコミット済みのものと同じかを確かめます |
| Dockerfile の `golang` | minor・major は、**自動の更新から外しています**。Go のバージョンは、`go.mod`・`mise.toml`・CI の `setup-go` と揃えて、手動で上げます(patch だけ、自動) |
| Terraform のプロバイダー | **本番の `terraform plan` は、Dependabot の PR では動きません**(Azure にログインする権限を、Dependabot に渡さないため)。マージ前に、手元で `terraform plan` を確認してください(`platform/terraform/envs/production/README.md`)。マージして `production` ブランチに入れると、通常どおり plan → 承認 → apply です |
| `next` / `react` | `next-react` グループで揃えて上がります。画面の動作は、`npm run build` と、実際の画面で確認してください |

## 変えたいとき

- **頻度・曜日**: `schedule`(`interval: daily` / `weekly` / `monthly`)。
- **PR の数**: `open-pull-requests-limit`。
- **まとめ方**: `groups`(`patterns` で、まとめる対象を指定)。
- **更新から外す**: `ignore`(例: 特定のライブラリ、または `update-types` でメジャーだけ)。
- **脆弱性の更新**: 上の設定とは別に、GitHub の **Dependabot security updates**(Settings > Code security)を有効にすると、脆弱性が見つかったときに、すぐに PR が作られます(週1回の確認を待ちません)。有効にしておくことをおすすめします。
