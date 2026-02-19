# FLOW（運用フロー）

## 基本方針

- PR は最後に作る（検証が通ってから）
- push で CI を回さない（回すなら最小）
- GitHub は“計算しない”。結果を受け取るだけ

## 推奨フロー

1) MacBook: 編集 + verify-lite（速い）
2) MacBook → Mac mini: verify-full（重い）
3) Mac mini: `verify-full` 実行後に `review-pack` で証拠bundle生成
4) MacBook: gh で PR 作成（1回だけ）
5) GitHub: verify-only（軽量）+ レビュー

## 実行契約（verify-lite / verify-full）

### verify-lite（ローカル軽量）

- 実行場所: Workstation（MacBook）
- 標準入口: `ops/ci/run_verify_lite.sh`
- 目的: 早い失敗検出（公式推奨lint + 単体テスト）
- Go公式推奨: `gofmt -l .` / `go vet ./...` / `go test ./...`
- 出力: `out/verify-lite.status` と `OK:/SKIP:/ERROR:` ログ

### verify-full（CI本番）

- 実行場所: CI Host（Mac mini）上の Docker コンテナ
- 標準入口: `ops/ci/run_verify_full.sh`
- 入力契約: `/repo` に対象リポジトリを mount する
- 出力契約: `/out` に `verify-full.status` と `logs/` を出力する
- キャッシュ契約: `/cache` を named volume として使う
- ステータス契約: 最後に `STATUS: OK|ERROR|SKIP` を1行で出力する
- ログ契約: 重要イベントは `OK:/SKIP:/ERROR:` で出力し、`out/verify-full.status` にも残す
- 実行モード:
  - 通常: `VERIFY_DRY_RUN=0 VERIFY_GHA_SYNC=0`
  - dry-run: `VERIFY_DRY_RUN=1`
  - GitHub Actions同期: `VERIFY_GHA_SYNC=1`（`GITHUB_*` 環境変数を status に記録）

### 標準 docker run（例）

```bash
docker run --rm \
  -v "$PWD:/repo" \
  -v "$PWD/out:/out" \
  -v ci-cache:/cache \
  -w /repo \
  ci-self-runner:local \
  /usr/local/bin/verify-full
```

### dry-run 実行例

```bash
VERIFY_DRY_RUN=1 ops/ci/run_verify_full.sh
```

### 証拠bundle生成（review-pack）

```bash
go run ./cmd/review-pack --profile core
go run ./cmd/review-pack --profile optional
```

## 例外

- 緊急修正は「lite→PR→full」は可。ただし runbook に理由を残す

## Discord通知

- CI失敗時のみ Discord 通知を送る（`DISCORD_WEBHOOK_URL` 使用）
- 通知内容は `repo/ref/sha/run_url` と最小ログに限定する
