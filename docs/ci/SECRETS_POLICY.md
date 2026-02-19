# SECRETS_POLICY

## 原則

- Webhook URL / token / PAT をリポジトリにコミットしない
- 共有は GitHub Actions Secrets 経由で行う
- 通知メッセージはテキスト中心にし、大容量ファイルは GitHub Run / Artifact へのリンクを使う

## 対象シークレット

- `DISCORD_WEBHOOK_URL`（`#ci-alerts`）
- `DISCORD_RELEASES_WEBHOOK_URL`（任意、`#releases`）
- `DISCORD_RUNBOOK_WEBHOOK_URL`（任意、通常は自動化しない）

## 漏えい時の対応（必須）

1. 漏えいした Webhook を Discord 側で削除
2. 新しい Webhook を再作成
3. GitHub Secret を即更新
4. 影響範囲（Issue/PR/ログ/チャット）を確認し、不要な露出を削除

## スキャン対象パターン

- `discord[.]com/api/webhooks/`
- `discordapp[.]com/api/webhooks/`（legacy）
- `hooks[.]slack[.]com/services/`（将来拡張）

## 禁止事項

- Webhook URL を README / docs / Issue / PR コメントに直接貼る
- CIログに Secret の生値を表示する
- `out/logs/` や `out/reviewpack/`（bundle含む）に Secret 生値を残す
