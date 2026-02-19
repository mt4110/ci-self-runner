# SECURITY_HARDENING_TASK

- [ ] T00 観測ログ採取: runner online状態、workflow実行ログ、`out/verify-full.status` を確認
- [ ] T01 外部PR防止: self-hosted job に fork/owner guard を設定
- [ ] T02 Action pin: `.github/workflows/*.yml` の `uses:` を commit SHA 固定
- [ ] T02.1 方針: 可能なものは内部Goコマンドを優先し、第三者Action依存を減らす
- [ ] T03 Token最小化: workflow `permissions` を read中心に維持
- [ ] T04 Secret運用: webhook/token を repo・ログ・PR本文へ出さない
- [ ] T04.1 Secret漏えい対策: review-pack / logs に webhook生値を含めない（scan対象に含める）
- [ ] T05 コンテナ隔離: 実処理は docker 経由、mountを `/repo,/out,/cache` に限定
- [ ] T05.1 Cache/Artifact汚染対策: cache key分離・artifact実行禁止を徹底
- [ ] T06 ネットワーク姿勢: allowlist か監視方針を明記（未実装なら理由を `SKIP` で記録）
- [ ] T07 緊急停止手順: RUNBOOKに runner 停止・tokenローテ・再登録手順を保持

## 検証コマンド

```bash
rg -n "pull_request_target|runs-on:|uses:" .github/workflows/verify.yml
mise x -- go run ./cmd/verify-lite
rg -n "single-owner|外部PR|緊急停止|cache|artifact|third-party action|internal Go" README.md docs/ci/RUNBOOK.md docs/ci/SECURITY_HARDENING_*.md
```

## Local Evidence (2026-02-19)

- `mise x -- go test ./...` -> `OK`（全パッケージテスト/ビルド成功）
- `mise x -- go run ./cmd/verify-lite` -> `OK`（secret_scan, workflow_policy_scan, go_checks）
- `DISCORD_WEBHOOK_URL='' mise x -- go run ./cmd/notify_discord --status out/verify-full.status --min-level ERROR` -> `ERROR: notify_discord webhook_missing ...`（終了は継続）
- `DISCORD_WEBHOOK_URL='https://example.invalid/webhook' mise x -- go run ./cmd/notify_discord --dry-run --status out/verify-full.status --min-level ERROR` -> `SKIP: notify_discord level=OK min=ERROR`
