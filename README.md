# runner-kit (self-hosted runner + colima + docker)

※Mac OS only

GitHub を「計算機」ではなく「公証台帳」に寄せる運用キットです。  
検証の主処理は Mac mini（self-hosted + colima + docker）で実行し、PRは最後に作成します。

## Scope（重要）

- このリポジトリは **個人運用（single-owner）専用** です
- self-hosted runner は **自分のリポジトリ/自分の変更** に限定して使います
- 外部コラボ・外部PR・fork PR の実行用途は想定しません
- 上記を外れて運用する場合は、`docs/ci/SECURITY_HARDENING_TASK.md` を先に満たしてください
- GitHub Actions の self-hosted 実行は `SELF_HOSTED_OWNER` 変数一致時のみ有効です

## Production QuickStart（実稼働用）

詳細: `docs/ci/QUICKSTART.md`

```bash
# 1) Runner セットアップ（初回のみ・冪等）
go run ./cmd/runner_setup --apply

# 2) 健康診断
go run ./cmd/runner_health

# 3) 軽量検証（ホストラッパ経由）
go run ./cmd/verify_lite_host

# 4) フル検証 dry-run（ホストラッパ経由）
go run ./cmd/verify_full_host --dry-run
```

SOT（判定の真実）: `out/runner-setup.status`, `out/health.status`, `out/verify-lite.status`, `out/verify-full.status`

## 初学者向け: 安全に始める3ステップ

1. GitHubの変数/シークレットを先に設定する（これをしないと self-hosted job は動かない）  
2. ローカルで `verify-lite` -> `verify-full --dry-run` の順に実行する  
3. 問題が出たら `out/verify-lite.status` / `out/verify-full.status` の `ERROR:` 行から確認する

最初に1回だけ実行:

```bash
# 変数（owner名）
gh variable set SELF_HOSTED_OWNER -b "$(gh repo view --json owner --jq .owner.login)" -R <owner/repo>

# 失敗通知（任意）
printf '%s' '<paste-discord-webhook-url-here>' | gh secret set DISCORD_WEBHOOK_URL -R <owner/repo>
```

## system architecture flow

![system architecture](docs/assets/systemArchitecture.png)  

## 入口ドキュメント

- `docs/ci/QUICKSTART.md`（実稼働 QuickStart）
- `docs/ci/QUICKSTART_PLAN.md`（設計 SOT）
- `docs/ci/RUNNER_LOCK.md`（Runner バージョン固定）
- `docs/ci/SYSTEM.md`
- `docs/ci/FLOW.md`
- `docs/ci/RUNNER_ISOLATION.md`
- `docs/ci/COLIMA_TUNING.md`
- `docs/ci/SHELL_POLICY.md`
- `docs/ci/SECRETS_POLICY.md`
- `docs/ci/DISCORD_NOTIFICATIONS.md`
- `docs/ci/GARBAGE_COLLECTION.md`
- `docs/ci/RUNBOOK.md`
- `docs/ci/SECURITY_HARDENING_PLAN.md`
- `docs/ci/SECURITY_HARDENING_TASK.md`

## 前提セットアップ（初回）

```bash
mise trust
mise install
```

## Quick Start（最短）

```bash
# 1) 軽量検証（公式推奨: gofmt/vet/test）
mise x -- go run ./cmd/verify-lite

# 2) フル検証（dry-run）
mkdir -p out cache
REPO_DIR='.' OUT_DIR='out' CACHE_DIR='cache' mise x -- go run ./cmd/verify-full --dry-run

# 3) レビューパック（core）
mise x -- go run ./cmd/review-pack --profile core

# 4) optional版レビューパック（必要時のみ）
mise x -- go run ./cmd/review-pack --profile optional

# 5) Discord通知のdry-run（Webhook送信なし）
DISCORD_WEBHOOK_URL='https://example.invalid/webhook' mise x -- \
  go run ./cmd/notify_discord --dry-run --status out/verify-full.status --title "verify-full local" --min-level ERROR
```

## 実行フロー（推奨）

1. MacBookで `verify-lite`
2. Mac miniで `verify-full`（または `remote_verify --mode remote`）
3. `review-pack --profile core` で提出パック生成
4. 必要時のみ `review-pack --profile optional`
5. 検証後にPR作成（GitHubは証跡の公証台帳）

## ローカル/リモート実行

### verify-full（ローカル）

`verify-full.status` に `GITHUB_RUN_ID / GITHUB_SHA / GITHUB_REF_NAME` を記録したい場合:

```bash
go run ./cmd/remote_verify --mode local
```

生成物:

- `out/verify-full.status`
- `out/logs/`
- 実行後に `out/logs` は自動で最新5件に整理

### verify-full（MacBook -> SSH -> Mac mini）

```bash
go run ./cmd/remote_verify \
  --mode remote \
  --remote-host <ssh-host-alias> \
  --remote-repo <remote-repo-path>
```

回収される生成物:

- `out/remote/verify-full.status`
- `out/remote/logs/`

## CIオーケストレーター（Go）

```bash
go run ./cmd/ci_orch run-plan --timebox-min 20
```

個別ステップ実行:

```bash
go run ./cmd/ci_orch preflight
go run ./cmd/ci_orch verify-lite
go run ./cmd/ci_orch full-build
go run ./cmd/ci_orch full-test
go run ./cmd/ci_orch bundle-make
```

## Discord通知（ローカル確認）

- 通知は外部Actionを使わず `cmd/notify_discord`（Go）で送信
- 既定は `--min-level ERROR`（ERROR時のみ送信）

Secret設定（GitHub Actions）:

```bash
printf '%s' '<paste-discord-webhook-url-here>' | gh secret set DISCORD_WEBHOOK_URL -R <owner/repo>
```

ownerガード変数（必須）:

```bash
gh variable set SELF_HOSTED_OWNER -b "$(gh repo view --json owner --jq .owner.login)" -R <owner/repo>
```

dry-run（Webhook送信せずpayload確認）:

```bash
go run ./cmd/notify_discord --dry-run --status out/verify-full.status --title "verify-full local" --min-level ERROR
```

将来拡張（任意）:

- 基本はテキスト通知（run URL中心）
- 将来ログ添付を行う場合は、秘匿情報マスクとサイズ制限を先に定義してから有効化

## レビューパック（ChatGPT / Gemini）

### 必須（core）

```bash
go run ./cmd/review-pack --profile core
```

生成物:

- `out/reviewpack/review-pack-<UTC>.tar.gz`
- `out/reviewpack/latest.tar.gz`
- `out/reviewpack/review-pack-<UTC>/PACK_SUMMARY.md`
- `latest.tar.gz` はエイリアス/シンボリックリンクではなく実体ファイル（copy）です

### Optional（追加証跡を含む）

```bash
go run ./cmd/review-pack --profile optional
```

生成物:

- `out/reviewpack/review-pack-optional-<UTC>.tar.gz`
- `out/reviewpack/latest-optional.tar.gz`
- `out/reviewpack/review-pack-optional-<UTC>/PACK_SUMMARY.md`
- `latest-optional.tar.gz` も実体ファイル（copy）です
- 実行後に `out/reviewpack` は自動で最新5件に整理（`latest*.tar.gz` は保持）

実体確認コマンド:

```bash
ls -l out/reviewpack/latest.tar.gz out/reviewpack/latest-optional.tar.gz
file out/reviewpack/latest.tar.gz out/reviewpack/latest-optional.tar.gz
```

## GC（out配下の整理）

dry-run:

```bash
go run ./cmd/gc_out
```

既定: `out/logs` と `out/reviewpack` は最新5件保持。

実削除:

```bash
go run ./cmd/gc_out --apply --max-delete 50
```

## Git管理しないもの

- `out/`（ログ、status、reviewpack成果物）
- `.local/`（ローカル state / 実行履歴）
- `cache/`（ローカルキャッシュ）

## 公開前チェック（最短）

```bash
mise x -- go test ./...
mise x -- go run ./cmd/verify-lite
REPO_DIR='.' OUT_DIR='out' CACHE_DIR='cache' mise x -- go run ./cmd/verify-full --dry-run
mise x -- go run ./cmd/review-pack --profile core
```
