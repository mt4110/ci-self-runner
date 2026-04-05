# QUICKSTART

## 1) CLI を入れる（初回のみ）

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

## 2) CI対象リポジトリで 1 コマンド実行

```bash
cd ~/dev/<target-repo>
ci-self up
```

`ci-self up` の実行内容:

1. `register`（runner登録・health・owner変数・workflow/template雛形）
2. `run-focus`（verify実行/監視・PR checks監視・PRテンプレ同期）

## 2.5) GitHub権限なしで対象jobだけ計測する

```bash
brew install act
cd ~/dev/<target-repo>
ci-self act
ci-self act --list
ci-self act --job <job-id>

# どこからでも明示指定できる
ci-self act --project-dir ~/dev/<target-repo> --job <job-id>
```

**この計測値はローカルでの概算です。実際の GitHub Actions / `remote-ci` / 実機 self-hosted runner の所要時間とは異なる場合があります。**

- `ci-self act` は対象 repo の `.github/workflows/*.yml|*.yaml` を見る
- `--workflow` を省略すると、repo の `.github/workflows/*.yml|*.yaml` から選ぶ。複数ある場合は対話選択、`q` で終了
- まず `ci-self act --list` で job id を確認してから `--job <job-id>` を付ける
- workflow 選択画面の番号と `--job` は別物。`--job` には `verify` のような job id を入れる
- `~/dev/maakie-brainlab` なら `ci-self act --project-dir ~/dev/maakie-brainlab --list` のあと `ci-self act --project-dir ~/dev/maakie-brainlab --job verify`
- `gh auth` や `SELF_HOSTED_OWNER` が無くても回せる
- 実行時間は `elapsed_sec` に加えて `benchmark_started_at` / `benchmark_finished_at` を出し、artifact は `out/act-artifacts/` に出す
- 実行中ログは左端に `[YYYY MM/DD HH:MM:SS]` を付ける
- `verify-full-dryrun` は Docker/Colima が必要
- workflow が1つも無い repo では、まず `.github/workflows/*.yml` を置く
- 既存 workflow が古い場合は `bash ops/ci/scaffold_verify_workflow.sh --repo <target> --apply --force` で更新する
- workflow に `github.event.act == true` が無い場合、owner guard で job が skip されることがある
- TTY から `scaffold_verify_workflow.sh --apply` を叩くと、`verify.yml` の作成/上書き前に `[y/N]` を聞く

## 設定ファイルで毎回のオプションを省略

```bash
ci-self config-init
```

`.ci-self.env` 例:

```env
CI_SELF_REPO=<owner>/<repo>
CI_SELF_REF=main
CI_SELF_PROJECT_DIR=/Users/<you>/dev/<target-repo>
CI_SELF_REMOTE_HOST=<you>@ci-runner.local
CI_SELF_REMOTE_PROJECT_DIR=/Users/<you>/dev/<target-repo>
CI_SELF_REMOTE_IDENTITY=/Users/<you>/.ssh/id_ed25519_for_ci_runner
CI_SELF_PR_BASE=main
```

## 別端末の CI runner を使う

- マシンA: self-hosted runner / colima / docker を置く端末
- マシンB: そこへ verify を依頼する手元端末

比喩で言うと、マシンB は普段の机、マシンA は重い作業を引き受ける工房です。
`remote-ci` は、机の上の作業ツリーを工房へ運び、検証後の結果だけを机へ戻すイメージです。

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
3. （`--repo` 指定時かつ remote 側 `gh auth status` 成功時のみ）bootstrap 実行
4. マシンA 側 verify 実行
5. `out/remote/<host>/` へ結果回収

既定では `target/`, `dist/`, `node_modules/`, `.venv/`, `coverage/`, `.next/` などの生成物ディレクトリと `.git/` を同期せず、`rsync --info=progress2` で進捗を表示します。
ローカル `rsync` が古い場合は `-h --progress` に自動フォールバックしますが、Homebrew の新しい `rsync` を推奨します。
repo 側の build/test が Git メタデータを直接読む場合だけ `--sync-git-dir` を付けてください。
remote 側で GitHub CLI 未導入または未ログインなら bootstrap は skip されますが、verify 自体は続行します。

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
