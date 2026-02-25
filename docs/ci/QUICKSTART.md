# QUICKSTART（実稼働の最短導線）

## 最短2コマンド（推奨）

最初の1回だけ:

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

CI実施プロジェクトで:

```bash
cd ~/dev/maakie-brainlab
ci-self register
ci-self run-focus
```

同一LANの Mac mini で実行する場合:

```bash
cd ~/dev/maakie-brainlab
ci-self remote-up --host <mac-mini-host> --project-dir ~/dev/maakie-brainlab --repo mt4110/maakie-brainlab
```

外出先で SSH 可能な場合:

```bash
ci-self remote-up --host <mac-mini-remote-host> --project-dir ~/dev/maakie-brainlab --repo mt4110/maakie-brainlab
```

外出先で SSH なしの場合（dispatch/watch のみ）:

```bash
ci-self run-focus --repo mt4110/maakie-brainlab --ref main
```

## 前提

- macOS（Mac mini 推奨）
- `go`, `gh`, `docker`, `colima` がインストール済み
- `mise trust && mise install` 実行済み

## 1) Runner セットアップ（初回のみ）

```bash
# 再起動直後は runtime を復帰
colima status || colima start

# 1コマンドで runner 登録まで実行
go run ./cmd/runner_setup --apply --repo <owner/repo>
```

- SOT: `out/runner-setup.status` の `status=OK` を確認
- 冪等: 既にセットアップ済みならスキップ
- `RUNNER_TOKEN` 未指定時は `gh api` で registration token を自動取得

## 2) 健康診断

```bash
go run ./cmd/runner_health
```

- SOT: `out/health.status` の `status=OK` を確認
- runner online / colima / docker / disk を一括チェック

## 3) 軽量検証

```bash
go run ./cmd/verify_lite_host
```

- SOT: `out/verify-lite.status` の `status=OK` を確認

## 4) フル検証（dry-run）

```bash
go run ./cmd/verify_full_host --dry-run
```

- SOT: `out/verify-full.status` の `status=OK` を確認

## 5) フル検証（本番）

```bash
# docker image を先にビルド
docker build -t ci-self-runner:local -f ci/image/Dockerfile .

# フル検証を実行
go run ./cmd/verify_full_host
```

- SOT: `out/verify-full.status` の `status=OK` を確認

## トラブルシュート

`ERROR:` が出たら:

1. 該当の `out/*.status` ファイルを確認
2. `reason=` 行で原因を特定
3. `docs/ci/RUNBOOK.md` の復旧手順を参照

SSH経由の簡易復旧:

```bash
ci-self remote-register --host <mac-mini-host> --project-dir ~/dev/<repo> --repo <owner/repo>
ci-self remote-run-focus --host <mac-mini-host> --project-dir ~/dev/<repo> --repo <owner/repo> --ref main
```
