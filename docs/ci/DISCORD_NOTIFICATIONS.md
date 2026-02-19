# DISCORD_NOTIFICATIONS

## 目的

GitHub メール通知を減らし、CI の失敗通知を Discord に集約する。

## 使用する Secret 名

- 必須: `DISCORD_WEBHOOK_URL`（`#ci-alerts`）
- 任意: `DISCORD_RELEASES_WEBHOOK_URL`（`#releases`）
- 任意: `DISCORD_RUNBOOK_WEBHOOK_URL`（通常は手動運用）

## Quick Setup

1. Discord で `#ci-alerts` 用 Webhook を作成
2. ローカルで以下を実行（URLは標準入力から設定）

```bash
printf '%s' '<paste-discord-webhook-url-here>' | gh secret set DISCORD_WEBHOOK_URL -R <owner/repo>
```

3. 確認

```bash
gh secret list -R <owner/repo>
```

## Workflow 通知ルール

- `verify` workflow は失敗時のみ Discord 送信
- 通知には `repo / ref / sha / run URL` を含める
- Secret 未設定時は通知ステップを `SKIP` する

## GitHub 通知ノイズ削減

- GitHub の Notification settings で email を抑制し、Web/Discord中心に切り替える
- `Watching` を必要最小にする
- `Failed workflows` を優先して確認する

## セキュリティ注意

- Webhook URL を repo / docs / issue / PR に貼らない
- 漏えい時は `docs/ci/SECRETS_POLICY.md` のローテーション手順を実施する
