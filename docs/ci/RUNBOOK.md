# RUNBOOK（壊れた時のロールバック / 復旧）

## 目的

runner/colima が壊れても「原因が追える」「戻せる」こと。

## Mac mini runner 初期セットアップ（最小）

### 1) 専用ユーザと配置

- runner は `ci` ユーザで実行し、開発ユーザと分離する
- 作業ディレクトリ例: `/Users/ci/ci-root/runner`

### 2) GitHub runner インストール

```bash
mkdir -p /Users/ci/ci-root/runner && cd /Users/ci/ci-root/runner
# GitHub Releases から actions runner を取得して展開
./config.sh --url <REPO_OR_ORG_URL> --token <TOKEN> \
  --labels self-hosted,mac-mini,colima,verify-full \
  --runnergroup Default \
  --unattended
./svc.sh install
./svc.sh start
```

### 3) ラベル方針

- `verify-full` ジョブは `self-hosted,mac-mini,colima,verify-full` を要求する
- 汎用ジョブにこのラベルを付けない（占有を防ぐ）

### 4) 禁止事項（セキュリティ）

- fork / 外部PRのジョブを self-hosted runner で実行しない
- 不明ソースのスクリプトを `ci` ユーザ権限で直接実行しない

## よくある症状と対処

### 1) runner が落ちる/ジョブ拾わない

- GitHub側: runner の Online/Offline 確認
- Mac mini側: runner プロセス、ログ確認
- まず: “再起動” より “ログ採取” を優先

### 2) colima が不安定/遅い

- disk不足/キャッシュ肥大/ファイルI/O を疑う
- まず: cache/out の容量確認、古い生成物を退避

## ロールバック方針

- 変更は小さく刻む（復旧しやすい）
- Docker image はタグ運用（以前のタグに戻せる）
- colima 設定値は docs/ci/COLIMA_TUNING.md に記録してから変更

## 外部PR防止（セキュリティ）

- self-hosted runner で外部PRを実行しない（fork PRは拒否）
- 本リポは single-owner 前提で運用する（外部コラボ用途に拡張しない）

## 緊急停止（止血）

インシデント疑い時は、原因調査より先に runner を停止する。

1. GitHub UI で runner を `Offline/Disable` にする
2. Mac mini 側で runner サービスを停止する
3. Discord/GitHub token をローテーションする
4. `docs/ci/HOST_CHANGELOG_TEMPLATE.md` に時刻と対応内容を残す
5. 復旧は「原因特定 -> 設定修正 -> runner再登録」の順で行う

## ChatGPT共有用レビューパック（期限なし）

### 目的（レビューパック）

- 現状を ChatGPT に渡すためのローカル `tar.gz` を作る
- クラウドの期限付きURLではなく、手元ファイルとして保持する

### 生成コマンド（core）

```bash
go run ./cmd/review-pack --profile core
```

### 生成コマンド（optional）

```bash
go run ./cmd/review-pack --profile optional
```

### 生成物

- core:
  - `out/reviewpack/review-pack-<UTC>.tar.gz`
  - `out/reviewpack/latest.tar.gz`
- optional:
  - `out/reviewpack/review-pack-optional-<UTC>.tar.gz`
  - `out/reviewpack/latest-optional.tar.gz`
- tar内に `PACK_SUMMARY.md`、`manifest.json`、`files/` が入る

### 共有手順

1) まず `out/reviewpack/latest.tar.gz`（core）を渡す
2) 追加証跡が必要なら `out/reviewpack/latest-optional.tar.gz` も渡す
3) `PACK_SUMMARY.md` を先に読むよう指示する
4) 必要に応じて `manifest.json` と `files/` を読ませる

### `latest*.tar.gz` 実体確認

`latest.tar.gz` / `latest-optional.tar.gz` はシンボリックリンクではなく、実体ファイルとして複製される。

```bash
ls -l out/reviewpack/latest.tar.gz out/reviewpack/latest-optional.tar.gz
file out/reviewpack/latest.tar.gz out/reviewpack/latest-optional.tar.gz
```

## Goオーケストレーター（ci_orch）

### 目的（ci_orch）

- 基盤処理の司令塔をGoで管理する（状態・分割実行・監査ログ）
- 実処理は既存Goコマンドを外部実行して責務分離する

### 実行コマンド

```bash
go run ./cmd/ci_orch preflight
go run ./cmd/ci_orch verify-lite
go run ./cmd/ci_orch full-build
go run ./cmd/ci_orch full-test
go run ./cmd/ci_orch bundle-make
go run ./cmd/ci_orch pr-create
go run ./cmd/ci_orch run-plan --timebox-min 20
```

### 契約

- 出力は必ず `OK: / SKIP: / ERROR:` の1行を含む
- 終了コードではなく出力行と state を成否の正とする
- state: `.local/ci/state.json`
- stepログ: `.local/out/run/<run_id>/<step>.log`

### 分割と停止

- `run-plan` は `preflight -> verify-lite -> full-build -> full-test -> bundle-make -> pr-create` の順で実行
- `ERROR` が出たら `STOP=true` とし、後続は `SKIP` で記録して進めない
- timebox超過は `SKIP: reason=timebox_exceeded` とする

### 実行前セットアップ（ローカル）

```bash
mise trust
mise install
```

- `go` は `mise.toml` の指定バージョンで揃える
- ローカル開発は `mise.toml`、コンテナ実行系は `ci/image/versions.lock` を正とする
- `ci_orch preflight` は不足コマンド（docker/go）を事前検知する

## GitHub Actions 同期

- workflow: `.github/workflows/verify.yml`
- `verify-lite` は公式lintを実行する
  - Go: `gofmt -l .` / `go vet ./...` / `go test ./...`
- `verify-full-dryrun` は self-hosted で `VERIFY_DRY_RUN=1` と `VERIFY_GHA_SYNC=1` を使う
- fork PR は self-hosted ジョブを実行しない

## 実行時パラメータ（運用）
- `verify-lite` の全体タイムアウトは `VERIFY_LITE_TIMEOUT_SEC` で指定する（既定: 600秒）
- `ops/ci/run_verify_full.sh` は既定でホストUID/GIDを使って `docker run --user` を設定する
- 必要に応じて `HOST_UID` / `HOST_GID` を明示指定できる
