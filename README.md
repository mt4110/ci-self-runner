# runner-kit (self-hosted runner + colima + docker)

macOS 向けの self-hosted runner 運用キットです。

## 最短導線

最初の1回だけ:

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

CI対象リポジトリで（ローカル）:

```bash
cd ~/dev/maakie-brainlab
ci-self up
```

- `ci-self up` は `register + run-focus` を連続実行
- `verify.yml` / PRテンプレートが無ければ自動雛形を生成

## ネットワーク別の最短

同一LANの Mac mini へ SSH:

```bash
ci-self remote-up --host <mac-mini-host> --project-dir ~/dev/maakie-brainlab --repo mt4110/maakie-brainlab
```

外出先（SSHあり）:

```bash
ci-self remote-up --host <mac-mini-remote-host> --project-dir ~/dev/maakie-brainlab --repo mt4110/maakie-brainlab
```

外出先（SSHなし）:

```bash
ci-self run-focus --repo mt4110/maakie-brainlab --ref main
```

## さらに短縮する設定ファイル

`ci-self` は `.ci-self.env` を自動読み込みします。

作成:

```bash
ci-self config-init
```

例:

```env
CI_SELF_REPO=mt4110/maakie-brainlab
CI_SELF_REF=main
CI_SELF_PROJECT_DIR=/Users/<you>/dev/maakie-brainlab
CI_SELF_REMOTE_HOST=mac-mini.local
CI_SELF_REMOTE_PROJECT_DIR=~/dev/maakie-brainlab
CI_SELF_PR_BASE=main
```

以後はオプションを減らして実行できます。

## 主要コマンド

- `ci-self up`: ローカル最短（register + run-focus）
- `ci-self focus`: run-focus 後、PR未作成なら自動作成し checks を監視
- `ci-self doctor --fix`: 依存/gh auth/colima/docker/runner_health を診断し可能な範囲で修復
- `ci-self remote-up`: SSH先で register + run-focus
- `ci-self config-init`: `.ci-self.env` テンプレート生成

注: `doctor --fix` は `gh auth login` だけは自動化できないため、未ログイン時は手動ログインが必要です。

## 初回セットアップ（対象リポジトリ）

```bash
# 必須: ownerガード
gh variable set SELF_HOSTED_OWNER -b "$(gh repo view --json owner --jq .owner.login)" -R <owner/repo>

# 任意: 失敗通知
printf '%s' '<discord-webhook-url>' | gh secret set DISCORD_WEBHOOK_URL -R <owner/repo>

# verify.yml が未作成なら（404回避）
bash ops/ci/scaffold_verify_workflow.sh --repo ~/dev/<target-repo> --apply
```

## セキュリティ前提

- 個人運用（single-owner）向け
- self-hosted 実行は `SELF_HOSTED_OWNER` 一致時のみ許可
- 外部コラボ / fork PR で使う場合は先に `docs/ci/SECURITY_HARDENING_TASK.md` を実施

## 詳細

- `docs/ci/QUICKSTART.md`
- `docs/ci/RUNBOOK.md`
- `docs/ci/SECURITY_HARDENING_TASK.md`
