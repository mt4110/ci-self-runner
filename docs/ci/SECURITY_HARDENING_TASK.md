# SECURITY_HARDENING_TASK

- [ ] T00 観測ログ採取: runner online状態、workflow実行ログ、`out/verify-full.status` を確認
- [ ] T01 外部PR防止: self-hosted job に fork/owner guard を設定
- [ ] T02 Action pin: `.github/workflows/*.yml` の `uses:` を commit SHA 固定
- [ ] T03 Token最小化: workflow `permissions` を read中心に維持
- [ ] T04 Secret運用: webhook/token を repo・ログ・PR本文へ出さない
- [ ] T05 コンテナ隔離: 実処理は docker 経由、mountを `/repo,/out,/cache` に限定
- [ ] T06 ネットワーク姿勢: allowlist か監視方針を明記（未実装なら理由を `SKIP` で記録）
- [ ] T07 緊急停止手順: RUNBOOKに runner 停止・tokenローテ・再登録手順を保持

## 検証コマンド

```bash
rg -n "pull_request_target|runs-on:|uses:" .github/workflows/verify.yml
mise x -- go run ./cmd/verify-lite
rg -n "single-owner|外部PR|緊急停止|stop" README.md docs/ci/RUNBOOK.md docs/ci/SECURITY_HARDENING_*.md
```
