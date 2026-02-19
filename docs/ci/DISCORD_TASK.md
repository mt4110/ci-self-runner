# DISCORD_TASK

- [x] 01. `docs/ci/DISCORD_PLAN.md` と本ファイルを同期
- [x] 02. `docs/ci/SECRETS_POLICY.md` を追加（Webhook漏えい時のローテーション手順を含む）
- [x] 03. `docs/ci/DISCORD_NOTIFICATIONS.md` を追加（`DISCORD_WEBHOOK_URL` と `gh secret set` を記載）
- [x] 04. `cmd/verify-full/main.go` から `os.Exit` を除去し、`OK:/ERROR:` を出力
- [x] 05. `cmd/verify-lite/main.go` から `os.Exit` を除去し、Webhookパターンスキャンを追加
- [x] 06. `cmd/notify_discord/main.go` を追加（`--status`, `--title`, `--dry-run`, `--webhook-env`, `--min-level`）
- [x] 07. `.github/workflows/verify.yml` を自前Go通知へ統一（外部Discord Action不使用）
- [x] 08. `docs/ci/FLOW.md` と `README.md` に通知/ログ契約を追記
- [x] 09. `mise x -- go test ./...` を実行
- [x] 10. `mise x -- go run ./cmd/review-pack --profile core` を実行
- [x] 11. `mise x -- go run ./cmd/review-pack --profile optional` を実行
- [x] 12. `mise x -- go run ./cmd/notify_discord --dry-run --status out/verify-full.status` を実行
