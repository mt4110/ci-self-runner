# QUICKSTART_TASK（実行順序固定・重い処理は分割）

## タスクリスト

1. [x] `docs/ci/QUICKSTART_PLAN.md` を追加（本設計の SOT）
2. [x] `docs/ci/QUICKSTART_TASK.md` を追加（実行順序固定）
3. [x] `docs/ci/QUICKSTART.md` を追加（利用者向け1枚手順）
4. [x] `docs/ci/RUNNER_LOCK.md` を追加（runner v2.331.0 + sha256 を固定）
5. [ ] `cmd/verify_lite_host/main.go` を追加
   - `out/verify-lite.status` が無い場合でも必ず生成（ERROR で）
   - 判定は `status=OK|ERROR|SKIP` と `ERROR:` 行
6. [ ] `cmd/verify_full_host/main.go` を追加
   - docker 実行前に `out/verify-full.status` を一旦退避/削除（前回混入防止）
   - docker 失敗でも必ず `out/verify-full.status` を生成（ERROR で）
7. [ ] `ops/ci/run_verify_lite.sh` を host wrapper 呼び出しに変更
8. [ ] `ops/ci/run_verify_full.sh` を host wrapper 呼び出しに変更（Shell 極薄維持）
9. [ ] `cmd/runner_setup/main.go` を追加（冪等セットアップ）
   - アーキ判定（osx-x64 / osx-arm64）
   - DL → sha256 検証 → 展開
   - config.sh --unattended --labels self-hosted,mac-mini,colima,verify-full
   - svc.sh install / svc.sh start
   - token はログに出さない
10. [ ] `cmd/runner_health/main.go` を追加（健康診断）
    - colima status / docker info の最小観測
    - out 容量、.local 容量、cache 容量の観測
    - 結果は `out/health.status`（OK/ERROR/SKIP）
11. [ ] `cmd/ci_orch/steps.go` を修正
    - `Process.Kill` を削除（強制終了禁止）
    - 外部コマンド exit code で判定しない
    - 各 step は status ファイルを読んで OK/ERROR/SKIP を確定
12. [ ] `README.md` に QuickStart（新導線）を追加
13. [ ] 軽い検証: `go test ./...`
14. [ ] 軽い検証: `go vet ./...`
15. [ ] 軽い検証: `gofmt -l .`
