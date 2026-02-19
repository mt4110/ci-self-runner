# DISCORD_NOTIFICATIONS

## 目的

GitHub メール通知を減らし、CI の失敗通知を Discord に集約する。
外部Discord Actionは使わず、`cmd/notify_discord`（Go）で通知する。

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

- `verify` workflow は `cmd/notify_discord` を実行する
- 既定は `--min-level ERROR` なので **ERROR時のみ送信**
- 通知には `repo / ref / sha / run URL` を含める
- Secret 未設定時は `ERROR: notify_discord webhook_missing ...` を出力し、ジョブは継続する（`continue-on-error: true`）

### verify.yml 例（短縮）

```yaml
- name: Notify Discord (CI Alerts)
  if: always()
  continue-on-error: true
  env:
    DISCORD_WEBHOOK_URL: ${{ secrets.DISCORD_WEBHOOK_URL }}
  run: go run ./cmd/notify_discord --status out/verify-full.status --title "verify-full-dryrun" --webhook-env DISCORD_WEBHOOK_URL --min-level ERROR
```

## GitHub 通知ノイズ削減

- GitHub の Notification settings で email を抑制し、Web/Discord中心に切り替える
- `Watching` を必要最小にする
- `Failed workflows` を優先して確認する

## セキュリティ注意

- Webhook URL を repo / docs / issue / PR に貼らない
- 漏えい時は `docs/ci/SECRETS_POLICY.md` のローテーション手順を実施する

## 将来拡張（任意）

- 基本はテキスト通知（run URL中心）を維持する
- 将来ログ添付を行う場合は、秘匿情報マスクとサイズ制限を先に設計する
