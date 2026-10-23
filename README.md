# runner-kit (self-hosted runner + colima + docker)

macOS 向けの self-hosted runner 運用キットです。

開発・テストには、依存ツールを `flake.lock` で固定できる **Nix環境を推奨**します。
本体の検証は `nix develop -c ci-self-test`、対象プロジェクトのworkflow実行は
`nix develop` 内から `sandbox --project-dir ...` を使います。
この推奨は本体と実行ツールの環境をそろえるためのものです。対象プロジェクトの依存関係は
元のworkflowに従い、workflowがNixを呼ぶ場合にだけNixで固定されます。

**ネイティブx86_64 Linux実機では未検証です。** 現在の実行実績はARM64 Mac上の開発シェル、
ARM64 Linux上の `postal_converter_ja` の `launcher` ジョブ、および命令変換を使うx86_64 Ubuntu VMでの同ジョブです。
`launcher` は `actions/setup-go` を使い、対象のNixジョブ全体の成功を示すものではありません。
VMでのNixジョブは長時間ビルドを中断しており、未完了です。
ARM64での結果は日常の事前チェックに使い、x86_64向けの最終確認はGitHub Actionsで行ってください。

## パスで指定したプロジェクトをUbuntu＋Nix内でCIテストする

`sandbox` は、対象のGitチェックアウトから作業コピーを作り、元のworkflowを
`act` でUbuntu＋Nixコンテナ内に実行します。プロジェクト専用の設定生成は不要です。

```bash
cd ~/dev/ci-self-runner
nix develop

# workflowが複数ある場合は選択を明示する
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --list
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test --plan
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test
```

`ci-self` が未導入なら、上の `ci-self` を `bash ops/ci/ci_self.sh` に置き換えられます。
実行にはDockerデーモン（macOSならColimaなど）が必要です。既存サービスを自動再起動しません。
既定ColimaのUnixソケットだけが不通でVMへSSH接続できる場合は、同じVMへの一時的なSSH転送を実行中だけ使います。
グローバルのDocker設定は変更しません。明示した `DOCKER_HOST` が不通の場合は自動代替しません。
`--plan` はファイルを作成せず互換性とイメージの対応を表示し、`--list` はジョブを表示します。
`--event` の既定は `push`。workflowが1つなら `--workflow` は省略できます。

- 対象は任意のGitリポジトリのルート。追跡済みファイルの現在の内容と、Gitに無視されていない未追跡ファイルをコピーします。削除済みファイルは復活させません。
- Git履歴もコピーしますが、元のGit設定・フックは引き継ぎません。`.env` / `.env.*`（`.env.example`以外）、`.secrets`、`.vars`、`.input`、`.actrc` はコピーせず、除外名を記録します。履歴内の秘密情報や、別名で保存された秘密情報を検出する機能ではありません。
- 作業コピーをコンテナへコピーし、元のプロジェクトはマウントしません。checkoutステップにはactのローカルコピー動作を使うため、未コミット変更をリモートHEADで上書きしません。
- workflowのコマンドは変更しません。Nixはコンテナ内で利用可能です。対象の `flake.nix` による依存固定は、workflow内の `nix develop` 等で利用します。非Nixのworkflowを自動でNixへ変換する機能ではありません。
- 既定の対応はUbuntu 22.04 / 24.04 / `ubuntu-latest`（24.04へ対応）。CPUは `linux/amd64`。`--arch linux/arm64` も選べますがx64とは別条件です。静的な `matrix.os` の展開と `needs` を確認します。matrixのexclude前の候補も保守的に検査します。
- macOS / Windows / 未知のself-hosted label、動的runner、再利用workflow、jobの `container:`、submoduleは自動代替せず停止します。`--job` を指定すれば、そのジョブと依存ジョブを検査・実行します。
- `--platform LABEL=BASE_IMAGE` でLinuxベースイメージを明示変更できます。例: `--platform ubuntu-24.04=ghcr.io/catthehacker/ubuntu:full-24.04`。full系は収録ツールを増やせますが、大きなダウンロードとディスク容量が必要です。VMの完全再現にはなりません。

結果は既定でユーザーキャッシュの `ci-self-runner/sandbox/run-*/` に保存します。
`--out /path/to/results` で変更できます（対象プロジェクトの外側を指定）。
`result.json` に実際のイメージdigest / ID、actのバージョン、ジョブの結果、失敗段階を記録し、
`run.log` と `artifacts/`、作業コピーも残します。全ジョブがスキップされた場合は非ゼロ終了します。
実行記録・作業コピー・永続キャッシュは自動削除せず、結果を自動公開しません。
actによる通常の成功時コンテナ削除は継続し、失敗・中断後に残った当該実行のコンテナは停止して保持します。

GitHubの権限、OIDC、environment承認、concurrency、VMのカーネルや常駐サービスは完全には再現できません。
このARM64 MacのColimaでは、x86_64エミュレーション内の `nix develop` はseccompエラーになります。
CPU差は警告・記録し、安全機構を無効化しません。x64のNixビルドにはx64のDocker実行先を使ってください。
actがjobのtimeoutを強制しないため、全体に `--timeout 45m`（変更可能）を適用します。
ホストの認証情報は自動注入せず、必要な場合だけ `--secret-file` / `--var-file` / `--event-file` を指定します。
既定のevent payloadには `act: true` を含めます。独自eventファイルはそのまま使用します。
ネットワークは有効で、artifact用のact HTTPサーバーも実行中に起動します。
Dockerソケットは既定で渡しません。Docker操作が必要な信頼済みworkflowだけ、ホストを操作できることを理解して `--docker-socket` を指定してください。

## ARM Macで日常の事前チェックを速く回す

既存のARM64 Colimaを使う場合は、CPU構成を明示します。Dockerの設定変更はそのコマンド内だけです。

```bash
nix develop
env -u DOCKER_HOST DOCKER_CONTEXT=colima bash ops/ci/ci_self.sh sandbox \
  --project-dir /path/to/project --workflow .github/workflows/ci.yml \
  --job test --arch linux/arm64 --timeout 10m
```

`--project-dir` とworkflow・ジョブを対象に合わせて変更できます。既定のCPUは引き続きx64です。
ARM64は事前チェックであり、GitHubのx64 runnerと同じ条件の合否には扱いません。
`postal_converter_ja` の元の `launcher` ジョブは、このMacで初回約51秒、再実行約11秒で成功しました。
[実行条件・計測結果](docs/ci/SANDBOX.md#native-arm64-preflight-on-this-mac) を参照してください。

## このMacでx86_64 Ubuntu VMを使う

ARM Macでもx64のLinuxカーネルを動かす専用VMを追加できます。既存Colimaとは別のLimaインスタンスです。

```bash
nix develop .#vm
ci-self vm start
ci-self sandbox --vm ci-self-x64 --nix-cache --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test
ci-self vm status
ci-self vm stop
```

準備済みのこのMacではそのまま利用できます。VM構成、初期導入の前提、実測結果は
[x86_64 VMの実行手順](docs/ci/X64_VM.md) を参照してください。
`--nix-cache` はVM内にプロジェクト別のNixストア・評価キャッシュを残し、次のジョブで再利用します。VMはジョブ終了後も起動したままです。
CPUの命令変換には時間がかかります。GitHub Actionsの完全再現や同等速度を保証しません。

## このプロジェクトをNixで開発・テストする

Nix（`nix-command` / `flakes` 有効）が導入済みなら、このチェックアウトで実行できます。

```bash
cd ~/dev/ci-self-runner
nix develop -c ci-self-test
```

`flake.lock` で固定したNixpkgsからGo 1.25系・Bash・Git・act・Docker CLIを用意し、
`gofmt` の確認、`go vet ./...`、`go test ./...` を順に実行します。
失敗した段階で非ゼロ終了します。初回は依存の取得が必要です。

対話的に開発する場合:

```bash
nix develop
go version
ci-self-test
# 個別テストも実行可能
go test ./ops/ci -run TestActHelp -v
```

プロジェクトは今の作業ディレクトリをそのまま使い、未コミット変更もテスト対象です。
新規の `flake.nix` / `flake.lock` が未追跡の間は、Gitの入力対象外になります。
`git add -N flake.nix flake.lock` で新規ファイルの存在をGitへ登録すると利用できます（コミットは不要）。
通常のGoキャッシュを再利用し、Goのツールチェーン自動取得は無効にしています。

これはホストOS上の開発環境です。VM・コンテナによる隔離やLinuxの再現ではありません。
対応定義はmacOS / LinuxのARM64・x86_64向けですが、実行確認済みの環境はmacOS ARM64です。
`ci-self-test` の単体テストにはSSH・GitHub・Docker等のモックを含みます。
実際のrunner登録・リモート接続・Docker / Colima起動の確認とは別です。

## 最短導線

最初の1回だけ:

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

CI対象リポジトリで（ローカル）:

```bash
cd ~/dev/<target-repo>
ci-self up
```

`ci-self act` で局所の概算時間を測る場合は、先に `brew install act` が必要です。

- `ci-self up` は `register + run-focus` を連続実行
- `verify.yml` / PRテンプレートが無ければ自動雛形を生成
- 雛形の生成はローカルファイル変更のみ（GitHub反映には commit/push が必要）
- 対象リポジトリに `flake.nix` がある場合、runner マシンに `nix` の事前インストールが必要
  - `ci-self` / `verify.yml` は `nix-daemon.sh` を自動読み込みして `nix` を検出（毎回の手動 `source` は不要）
  - 既存の `verify.yml` が古い場合は `bash ops/ci/scaffold_verify_workflow.sh --repo <target> --apply --force` で更新

## 別端末の CI runner を 1 コマンドで使う（推奨）

この導線は、特定のリポジトリ専用ではありません。

- マシンA: self-hosted runner / colima / docker を置いている端末
- マシンB: 普段コードを書く端末。ここからマシンAへ verify を依頼する

比喩で言うと、マシンB は普段の机、マシンA は重い作業を引き受ける工房です。
`remote-ci` は、机の上で書いたものを工房へ持ち込み、確認が終わった成果物だけを持ち帰る導線です。

マシンB から 1 コマンドで「鍵認証確認 -> 同期 -> マシンA実行 -> 結果回収」まで行います。

`remote-ci` の同期元は、現在の作業リポジトリです。対象 repo のルートで実行するか、`--local-dir <path>` で明示してください。

`remote-ci` はローカル `rsync` コマンドをそのまま使います。macOS 既定の `openrsync` だと古く、進捗表示や互換性で不利です。

```bash
ci-self remote-ci --host <user>@<machine-a-host> -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<project>' --repo <owner>/<repo>
```

例:

```bash
ci-self remote-ci --host ci@192.168.1.20 -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<target-repo>' --repo <owner>/<repo>
```

`remote-ci` の実行内容:

1. SSH 公開鍵認証（password禁止）を検証
2. マシンB のローカル作業ツリーをマシンA の `--project-dir` へ `rsync` 同期
3. （`--repo` 指定時かつ remote `ci-self` 利用可能時）マシンA 上で `ci-self register` をベストエフォート実行
4. マシンA で、同梱の verify wrapper を SSH 経由で実行
5. `verify-full.status` と `out/logs` をマシンB の `out/remote/<host>/` に回収

注:

- bootstrap は remote 側で `gh auth status` が通るときだけ実行する
- remote 側で GitHub CLI 未導入または未ログインなら、bootstrap は warning ではなく skip され、standalone verify は続行する

同期時の既定:

- `target/`, `dist/`, `node_modules/`, `.venv/`, `coverage/`, `.next/` などの生成物ディレクトリと `.git/` は同期しない
- `rsync --info=progress2` で進捗を表示する
- ローカル `rsync` が `--info=progress2` 非対応なら `-h --progress` へ自動フォールバックする
- build/test が Git メタデータを直接参照する repo だけは `--sync-git-dir` で `.git/` 同期を有効化する

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

### rsync バージョン注意

macOS 既定の `rsync` が古い場合があります。

```bash
rsync --version
```

古い例:

```text
openrsync: protocol version 29
rsync version 2.6.9 compatible
```

推奨:

```bash
brew install rsync
```

`alias rsync=...` だけでは `ci-self` の bash スクリプトには効かないことがあります。`PATH` の先頭に Homebrew 版を入れてください。

Apple Silicon の例:

```bash
echo 'export PATH="/opt/homebrew/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

確認例:

```text
rsync  version 3.4.1  protocol version 32
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
- `remote-ci` の同期元はデフォルトで「今いる repo」。別 repo を送りたい場合は `--local-dir` を使う
- `--project-dir` はマシンA 側の配置先パス。`~` を使う場合は `--project-dir '~/<path>'` のようにクオート
- `--local-dir` を使うと、マシンB 側の同期元を明示できる
- `--sync-git-dir` を付けると `.git/` も同期する。通常は不要
- `*.md`, `*.log`, `*.txt`, `build/`, `bin/`, `10MB超ファイル` は既定除外していない。tracked asset / fixture / source の可能性があるため
- remote `ci-self` 未導入でも verify 自体は動く。影響するのは bootstrap だけ
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
CI_SELF_REPO=<owner>/<repo>
CI_SELF_REF=main
CI_SELF_PROJECT_DIR=/Users/<you>/dev/<target-repo>
CI_SELF_REMOTE_HOST=<you>@ci-runner.local
CI_SELF_REMOTE_PROJECT_DIR=/Users/<you>/dev/<target-repo>
CI_SELF_REMOTE_IDENTITY=/Users/<you>/.ssh/id_ed25519_for_ci_runner
CI_SELF_PR_BASE=main
```

以後はオプションを減らして実行できます。

## GitHub権限なしで verify を絞って計測する

`gh workflow run` を使わず、ローカルで選んだ job だけを回して、局所の概算時間を見たいときは `act` 導線を使えます。

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

ポイント:

- `ci-self act` は対象 repo の `.github/workflows/*.yml|*.yaml` を見る
- `--workflow` を省略すると、対象 repo の `.github/workflows/*.yml|*.yaml` を見て、複数ある場合は `> どのworkflowを、actで実行したいですか？` と対話選択する
- 選択画面では `q` で抜けられる
- まず `ci-self act --list` で job id を確認してから、`--job <job-id>` を付ける
- workflow 選択画面の番号と `--job` は別物。`--job` には基本的に `verify` / `verify-lite` のような job id を渡す
- `~/dev/maakie-brainlab` なら `ci-self act --project-dir ~/dev/maakie-brainlab --list` のあと、`ci-self act --project-dir ~/dev/maakie-brainlab --job verify`
- 実行時間は `elapsed_sec` に加えて `benchmark_started_at` / `benchmark_finished_at` も出し、artifact は `out/act-artifacts/` に出す
- 実行中ログは左端に `[YYYY MM/DD HH:MM:SS]` を付けて流す
- `SELF_HOSTED_OWNER` や `gh auth` が無くても回せる
- workflow が1つも無い repo では、まず `.github/workflows/*.yml` を用意する必要がある
- 既存 workflow が古い場合は `bash ops/ci/scaffold_verify_workflow.sh --repo <target> --apply --force` で act 互換の verify.yml に更新する
- TTY から `scaffold_verify_workflow.sh --apply` を実行した場合は、`verify.yml` の新規作成/上書き前に `[y/N]` 確認が入る

注意:

- `act` は GitHub Actions の完全再現ではない。局所の概算時間測定と早い失敗検出には向くが、最終判定は `remote-ci` / 実機 self-hosted runner を優先する
- 既存 workflow に `github.event.act == true` の逃がしが無い場合、owner guard で job が skip されることがある
- `verify-full-dryrun` は手元の Docker/Colima 到達性が前提

## 主要コマンド

- `ci-self up`: ローカル最短（register + run-focus）
- `ci-self act`: `act` で verify workflow/job をローカル実行し、対象を絞って概算時間を測る
- `ci-self focus`: run-focus 後、PR未作成なら自動作成し checks を監視
- `ci-self mobile-workflow`: fastlane前提の `mobile-build.yml` 雛形を生成
- `ci-self remote-ci`: 鍵必須・同期・別端末での verify 実行・結果回収を1コマンドで実行
- `ci-self doctor --fix`: 依存/gh auth/colima/docker/runner_health を診断し可能な範囲で修復
- `ci-self update`: runner 本体と周辺ツール（act/gh/colima 等）の更新候補を確認
- `ci-self update --apply`: 既に Homebrew 管理されている周辺ツールだけを明示更新
- `ci-self doctor --repo-dir <path>`: `flake.nix` リポジトリの Nix 到達性も含めて診断
- `ci-self remote-up`: SSH先で register + run-focus（同期しない旧導線）
- `ci-self config-init`: `.ci-self.env` テンプレート生成

注: `doctor --fix` は `gh auth login` だけは自動化できないため、未ログイン時は手動ログインが必要です。
注: runner 本体は GitHub Actions runner の自動更新に任せ、`ci-self update --apply` では強制更新しません。

## Mobile build（fastlane）

iOS / Android 用 runner label は `--mobile-profile` で追加できます。

```bash
ci-self register --mobile-profile ios
ci-self register --mobile-profile android
ci-self register --mobile-profile all
```

対象アプリ repo では、develop/main 向けの雛形を作れます。

```bash
ci-self mobile-workflow --apply
```

詳細:

- `docs/ci/MOBILE_LABELS.md`
- `docs/ci/MOBILE_SECRETS_POLICY.md`
- `docs/ci/MAC_BUILD_HOST_RUNBOOK.md`

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
