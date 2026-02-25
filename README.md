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

## 理想系（2コマンド運用）

最初の1回だけ（CLIインストール）:

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

### ローカル版（自分マシン Self-Hosted）

```bash
cd ~/dev/maakie-brainlab
ci-self up
```

### ローカルネットワーク編（MacBook -> 同一LANの Mac mini）

```bash
cd ~/dev/maakie-brainlab
ci-self remote-up --host <mac-mini-host> --project-dir ~/dev/maakie-brainlab --repo mt4110/maakie-brainlab
```

### リモートネットワーク編（外出先）

```bash
# どこからでも (SSHあり): Mac mini 上で register + run-focus を1コマンド実行
ci-self remote-up --host <mac-mini-remote-host> --project-dir ~/dev/maakie-brainlab --repo mt4110/maakie-brainlab

# どこからでも (SSHなし): dispatch + All Green確認 + PRテンプレ同期のみ実行
ci-self run-focus --repo mt4110/maakie-brainlab --ref main
```

サブコマンド要約:

- `ci-self up`: `register` + `run-focus` を連続実行（ローカル最短）
- `ci-self register`: colima確認 + runner登録 + health + `SELF_HOSTED_OWNER` + 必要なら `verify.yml` / PRテンプレ雛形
- `ci-self run-focus`: verify dispatch/watch + PR checks All Green待機 + PRテンプレ同期
- `ci-self remote-up`: SSH先で `register` と `run-focus` を連続実行

補足: `remote-*` は接続先Macに `ci-self`（`bash ops/ci/install_cli.sh` 実行済み）が必要です。

## 補助コマンド（3ステップ版 / CLI未導入時）

```bash
# 1) runner登録
go run ./cmd/runner_setup --apply --repo <owner/repo>

# 2) 健康診断
go run ./cmd/runner_health

# 3) verify実行
go run ./cmd/verify_lite_host
go run ./cmd/verify_full_host --dry-run
```

SOT（判定の真実）:

- `out/runner-setup.status`
- `out/health.status`
- `out/verify-lite.status`
- `out/verify-full.status`

## 初回セットアップ（対象リポジトリ）

```bash
# 必須: ownerガード
gh variable set SELF_HOSTED_OWNER -b "$(gh repo view --json owner --jq .owner.login)" -R <owner/repo>

# 任意: 失敗通知
printf '%s' '<discord-webhook-url>' | gh secret set DISCORD_WEBHOOK_URL -R <owner/repo>

# verify.yml が未作成なら（404回避）
bash ops/ci/scaffold_verify_workflow.sh --repo ~/dev/<target-repo> --apply
```

## 外出先運用の要点

- SSHあり: `ci-self remote-up ...` が最短
- SSHなし: `ci-self run-focus --repo <owner/repo> --ref main`
- runner/colima 停止時のみ、SSHで復旧

```bash
ci-self remote-register --host <mac-mini-host> --project-dir ~/dev/<repo> --repo <owner/repo>
ci-self remote-run-focus --host <mac-mini-host> --project-dir ~/dev/<repo> --repo <owner/repo> --ref main
```

## 詳細ドキュメント

- `docs/ci/QUICKSTART.md`（最短運用）
- `docs/ci/RUNBOOK.md`（障害復旧）
- `docs/ci/SECURITY_HARDENING_TASK.md`（外部コラボ時の必須対策）
- その他は `docs/ci/` 配下を参照
