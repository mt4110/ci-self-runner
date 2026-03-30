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

## 別端末の CI runner を 1 コマンドで使う（推奨）

この導線は、特定の `~/dev/maakie-brainlab` 専用ではありません。

- マシンA: self-hosted runner / colima / docker を置いている端末
- マシンB: 普段コードを書く端末。ここからマシンAへ verify を依頼する

マシンB から 1 コマンドで「鍵認証確認 -> 同期 -> マシンA実行 -> 結果回収」まで行います。

```bash
ci-self remote-ci --host <user>@<machine-a-host> -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<project>' --repo <owner>/<repo>
```

例:

```bash
ci-self remote-ci --host ci@192.168.1.20 -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/maakie-brainlab' --repo mt4110/maakie-brainlab
```

`remote-ci` の実行内容:

1. SSH 公開鍵認証（password禁止）を検証
2. マシンB のローカル作業ツリーをマシンA の `--project-dir` へ `rsync` 同期
3. （`--repo` 指定時）マシンA 上で `ci-self register` をベストエフォート実行
4. マシンA で `ops/ci/run_verify_full.sh` を実行
5. `verify-full.status` と `out/logs` をマシンB の `out/remote/<host>/` に回収

### 初回だけ必要な準備（マシンB -> マシンA）

1. マシンB で SSH 鍵を作る（未作成なら）

```bash
ssh-keygen -t ed25519 -a 100
```

2. マシンB の公開鍵をマシンA の `~/.ssh/authorized_keys` に登録する

```bash
cat ~/.ssh/id_ed25519_for_ci_runner.pub | ssh -i ~/.ssh/id_ed25519_for_ci_runner <user>@<machine-a-host> 'mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'
```

3. パスワードなし SSH を確認する

```bash
ssh -i ~/.ssh/id_ed25519_for_ci_runner -o BatchMode=yes -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no <user>@<machine-a-host> true
```

4. 通ったら `remote-ci` を実行する

```bash
ci-self remote-ci --host <user>@<machine-a-host> -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<project>' --repo <owner>/<repo>
```

公開鍵未登録時は、`remote-ci` 自体が `authorized_keys` 登録のヒントを出して停止します。

### 外出先からでも使える？

使える場合があります。`remote-ci` が必要としているのは「同一LAN」ではなく「SSH 到達性」です。

- 使える: マシンA に外出先から SSH で到達できる場合
- 典型例: Tailscale / VPN / ポート転送済みの自宅回線 / 固定IP など
- 使えない: マシンA へ SSH 経路が無い場合

`ci-self remote-ci` 自体は、外部公開やトンネル作成までは行いません。そこは別途ネットワーク設計が必要です。

### 今できること / まだできないこと

- できる: `--project-dir` と `--repo` を切り替えて、任意の CI 対象リポジトリをマシンA で実行
- できる: 未コミット変更を含むローカル作業ツリーを同期して verify を走らせる
- できる: `verify-full.status` と `out/logs` を手元へ回収する
- できる: 同一LANでも外出先でも、SSH 到達性があれば同じコマンドで使う
- まだできない: SSH パスワード認証での `remote-ci` 実行
- まだできない: SSH 経路が無い状態からの自動疎通確立
- まだできない: 複数プロジェクトの自動検出や自動振り分け。どの repo をどこで走らせるかは `--project-dir` / `.ci-self.env` で明示する

補足:

- `--host` は `ssh` の接続先文字列（`user@host` / IP / `~/.ssh/config` の Host 別名）
- `-i` / `--identity` で SSH 鍵ファイルを指定できる。毎回省略したい場合は `.ci-self.env` の `CI_SELF_REMOTE_IDENTITY` を使う
- `--project-dir` はマシンA 側の配置先パス。`~` を使う場合は `--project-dir '~/<path>'` のようにクオート
- `--local-dir` を使うと、マシンB 側の同期元を明示できる
- `--repo` を省略すると bootstrap は skip されるが、同期済みプロジェクト上での standalone verify は実行できる
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
CI_SELF_REMOTE_HOST=<you>@ci-runner.local
CI_SELF_REMOTE_PROJECT_DIR=/Users/<you>/dev/maakie-brainlab
CI_SELF_REMOTE_IDENTITY=/Users/<you>/.ssh/id_ed25519_for_ci_runner
CI_SELF_PR_BASE=main
```

以後はオプションを減らして実行できます。

## 主要コマンド

- `ci-self up`: ローカル最短（register + run-focus）
- `ci-self focus`: run-focus 後、PR未作成なら自動作成し checks を監視
- `ci-self remote-ci`: 鍵必須・同期・別端末での verify 実行・結果回収を1コマンドで実行
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
