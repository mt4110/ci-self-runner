# QUICKSTART（実稼働の最短導線）

## 前提

- macOS（Mac mini 推奨）
- `go`, `gh`, `docker`, `colima` がインストール済み
- `mise trust && mise install` 実行済み

## 1) Runner セットアップ（初回のみ）

```bash
go run ./cmd/runner_setup --apply
```

- SOT: `out/runner-setup.status` の `status=OK` を確認
- 冪等: 既にセットアップ済みならスキップ

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
