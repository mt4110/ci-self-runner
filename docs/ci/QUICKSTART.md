# QUICKSTART

## 1) CLI を入れる（初回のみ）

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

## 2) CI対象リポジトリで 1 コマンド実行

```bash
cd ~/dev/maakie-brainlab
ci-self up
```

`ci-self up` の実行内容:

1. `register`（runner登録・health・owner変数・workflow/template雛形）
2. `run-focus`（verify実行/監視・PR checks監視・PRテンプレ同期）

## 設定ファイルで毎回のオプションを省略

```bash
ci-self config-init
```

`.ci-self.env` 例:

```env
CI_SELF_REPO=mt4110/maakie-brainlab
CI_SELF_REF=main
CI_SELF_PROJECT_DIR=/Users/<you>/dev/maakie-brainlab
CI_SELF_REMOTE_HOST=<you>@ci-runner.local
CI_SELF_REMOTE_PROJECT_DIR=/Users/<you>/dev/maakie-brainlab
CI_SELF_REMOTE_IDENTITY=/Users/<you>/.ssh/id_ed25519_for_ci_runner
CI_SELF_PR_BASE=main
```

## 別端末の CI runner を使う

- マシンA: self-hosted runner / colima / docker を置く端末
- マシンB: そこへ verify を依頼する手元端末

まずマシンB で SSH 鍵を用意し、公開鍵をマシンA の `~/.ssh/authorized_keys` に登録します。

```bash
ssh-keygen -t ed25519 -a 100
cat ~/.ssh/id_ed25519_for_ci_runner.pub | ssh -i ~/.ssh/id_ed25519_for_ci_runner <user>@<machine-a-host> 'mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'
ssh -i ~/.ssh/id_ed25519_for_ci_runner -o BatchMode=yes -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no <user>@<machine-a-host> true
```

通ったら `remote-ci` を実行します。

`remote-ci` の同期元は、現在の作業リポジトリです。対象 repo のルートで実行するか、`--local-dir <path>` で明示してください。

```bash
ci-self remote-ci --host <user>@<machine-a-host> -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<project>' --repo <owner>/<repo>
```

`remote-ci` は以下を 1 コマンドで実行します:

1. SSH 鍵認証チェック（password不可）
2. ローカル変更をマシンA に `rsync` 同期
3. マシンA 側 verify 実行
4. `out/remote/<host>/` へ結果回収

`remote-ci` は LAN 専用ではなく、外出先でも SSH 到達性があれば使えます。
逆に SSH 経路が無い場合、`remote-ci` 自体は疎通を作れません。

runner 初期化/復旧専用の旧導線:

```bash
ci-self remote-up
```

外出先で SSH 到達性が無い場合:

```bash
ci-self run-focus
```

これは GitHub 側 workflow の dispatch/watch 用の別導線であり、マシンA での即時 `remote-ci` 実行そのものの代替ではありません。

## 事前診断と自己修復

```bash
ci-self doctor --fix
```

確認項目:

- `gh` / `colima` / `docker`
- `gh auth status`
- runner online（repo指定/設定時）
- `runner_health`

## トラブル時

- `ERROR:` が出たら `out/*.status` を確認
- 詳細手順は `docs/ci/RUNBOOK.md`
