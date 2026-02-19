# GATES（何をPASSとするか）

## verify-lite（MacBook）

- Go: `gofmt -l .` が空であること
- Go: `go vet ./...` が成功すること
- Go: `go test ./...` が成功すること

## verify-full（Mac mini + docker）

- 重いチェック（full test）
- 証拠bundle生成（logs / bundle sha）
- 失敗時は理由を1行で記録し、次のステップへ進まない
- dry-run は `VERIFY_DRY_RUN=1` で実行可能
- GitHub Actions同期は `VERIFY_GHA_SYNC=1` で有効化
