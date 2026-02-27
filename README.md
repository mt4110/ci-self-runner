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
- 雛形の生成はローカルファイル変更のみ（GitHub反映には commit/push が必要）
- 対象リポジトリに `flake.nix` がある場合、runner マシンに `nix` の事前インストールが必要
  - `ci-self` / `verify.yml` は `nix-daemon.sh` を自動読み込みして `nix` を検出（毎回の手動 `source` は不要）
  - 既存の `verify.yml` が古い場合は `bash ops/ci/scaffold_verify_workflow.sh --repo <target> --apply --force` で更新

## Mac mini ワンコマンド（推奨）

MacBook から 1 コマンドで「鍵認証確認 -> 同期 -> Mac mini 実行 -> 結果回収」まで行う:

```bash
ci-self remote-ci --host <user>@<mac-mini-ip-or-host> --project-dir '~/dev/maakie-brainlab' --repo mt4110/maakie-brainlab
```

`remote-ci` の実行内容:

1. SSH 公開鍵認証（password禁止）を検証
2. ローカル作業ツリーを Mac mini へ `rsync` 同期
3. （repo指定時）runner bootstrap をベストエフォート実行
4. Mac mini で `ops/ci/run_verify_full.sh` を実行
5. `verify-full.status` と `out/logs` をローカル `out/remote/<host>/` に回収

公開鍵未登録時は、`authorized_keys` 登録のヒントを出して停止します。

補足:

- `--host` は `ssh` の接続先文字列（`user@host` / IP / `~/.ssh/config` のHost別名）
- `--project-dir` に `~` を使う場合は `--project-dir '~/<path>'` のようにクオート
- runner 初期化/復旧専用の旧導線は `ci-self remote-up`

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
CI_SELF_REMOTE_HOST=<you>@mac-mini.local
CI_SELF_REMOTE_PROJECT_DIR=/Users/<you>/dev/maakie-brainlab
CI_SELF_PR_BASE=main
```

以後はオプションを減らして実行できます。

## 主要コマンド

- `ci-self up`: ローカル最短（register + run-focus）
- `ci-self focus`: run-focus 後、PR未作成なら自動作成し checks を監視
- `ci-self remote-ci`: 鍵必須・同期・Mac mini実行・結果回収を1コマンドで実行
- `ci-self doctor --fix`: 依存/gh auth/colima/docker/runner_health を診断し可能な範囲で修復
- `ci-self doctor --repo-dir <path>`: `flake.nix` リポジトリの Nix 到達性も含めて診断
- `ci-self remote-up`: SSH先で register + run-focus（同期しない旧導線）
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
