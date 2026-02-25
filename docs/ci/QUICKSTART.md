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
CI_SELF_REMOTE_HOST=mac-mini.local
CI_SELF_REMOTE_PROJECT_DIR=~/dev/maakie-brainlab
CI_SELF_PR_BASE=main
```

## ネットワーク別ワンコマンド

同一LAN / 外出先（SSHあり）:

```bash
ci-self remote-up
```

`remote-up` は `.ci-self.env` の `CI_SELF_REMOTE_HOST` などが設定済みの場合の最短です。
未設定なら `--host --project-dir --repo` を明示してください。

外出先（SSHなし）:

```bash
ci-self run-focus
```

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
