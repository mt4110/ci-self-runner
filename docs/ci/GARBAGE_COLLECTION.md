# GARBAGE_COLLECTION（GC / 破綻防止）

## 目的

self-hosted runner + colima + docker の劣化要因（ログ肥大、artifact蓄積、キャッシュ過多）を定期的に整理する。

## 原則

- 先に観測、次に削除（無思考 `prune` 禁止）
- `OK:` / `SKIP:` / `ERROR:` を必ず残す
- 重い削除は分割し、1回でやり切らない

## 対象

- `out/logs/`（TTL削除）
- `out/reviewpack/`（最新+5件保持）
- `out/gha-artifacts/`（直近N run_id保持）

## 実行コマンド（Go）

`verify-full` と `review-pack` は実行後に自動GC（最新5件保持）を行う。  
以下は手動で即時整理したい場合のコマンド。

### 1) 観測のみ（dry-run）

```bash
go run ./cmd/gc_out
```

- 既定: `out/logs` は最新5件保持、`out/reviewpack` は最新5件保持

### 2) 実削除（安全上限あり）

```bash
go run ./cmd/gc_out --apply --max-delete 50
```

### 3) 保持数/TTLの調整例

```bash
go run ./cmd/gc_out --apply --ttl-logs-days 14 --keep-logs 5 --keep-reviewpack 5 --keep-gha 10 --max-delete 50
```

## Docker/Colima側（観測優先）

```bash
docker system df
```

- prune は容量逼迫や障害時のみ
- 実施時は `docs/ci/RUNBOOK.md` に理由を追記する
