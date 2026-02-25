# QUICKSTART_PLAN（設計SOT）

## 目的

初回セットアップを「1コマンド＋2確認」で完了させる。
判定の唯一の真実（SOT）は `out/*.status` ファイル。exit code は使わない。

## 制約

- **exit 禁止**: 失敗は `ERROR:` を出力し、STOP フラグで後続をスキップする
- **kill 禁止**: `Process.Kill` による強制終了を設計から排除する
- **重い処理は分割**: docker build / verify-full / bundle 等は別ステップ
- **Shell は極薄**: `docs/ci/SHELL_POLICY.md` に準拠（Go 中心）

## 判定フロー（擬似コード）

```
state.stop = false
state.errors = []

step = "preflight_repo"
if not in_git_repo():
  print("ERROR: step=preflight_repo reason=not_in_git_repo")
  state.stop = true
else:
  print("OK: step=preflight_repo")

step = "preflight_tools"
if missing(any of ["gh","go","docker","colima"]):
  print("ERROR: step=preflight_tools reason=missing_tools")
  state.stop = true
else:
  print("OK: step=preflight_tools")

step = "runner_lock"
if state.stop:
  print("SKIP: step=runner_lock reason=STOP")
else:
  if runner_lock_invalid():
    print("ERROR: step=runner_lock reason=invalid_lock")
    state.stop = true
  else:
    print("OK: step=runner_lock version=v2.331.0")

step = "runner_setup"
if state.stop:
  print("SKIP: step=runner_setup reason=STOP")
else:
  result = run("go run ./cmd/runner_setup --apply")
  # SOT: out/runner-setup.status
  if result.status != "OK":
    print("ERROR: step=runner_setup reason=failed")
    state.stop = true
  else:
    print("OK: step=runner_setup")

step = "colima_ready"
if state.stop:
  print("SKIP: step=colima_ready reason=STOP")
else:
  result = run("go run ./cmd/runner_health --mode colima-ready")
  if result.status != "OK":
    print("ERROR: step=colima_ready reason=failed")
    state.stop = true
  else:
    print("OK: step=colima_ready")

step = "smoke_verify_lite"
if state.stop:
  print("SKIP: step=smoke_verify_lite reason=STOP")
else:
  result = run("go run ./cmd/verify_lite_host")
  # SOT: out/verify-lite.status
  if result.status != "OK":
    print("ERROR: step=smoke_verify_lite reason=verify_lite_error")
    state.stop = true
  else:
    print("OK: step=smoke_verify_lite")

step = "build_image"
if state.stop:
  print("SKIP: step=build_image reason=STOP")
else:
  result = run("docker build -t ci-self-runner:local -f ci/image/Dockerfile .")
  if result.status != "OK":
    print("ERROR: step=build_image reason=build_failed")
    state.stop = true
  else:
    print("OK: step=build_image")

step = "smoke_verify_full_dryrun"
if state.stop:
  print("SKIP: step=smoke_verify_full_dryrun reason=STOP")
else:
  result = run("go run ./cmd/verify_full_host --dry-run")
  # SOT: out/verify-full.status
  if result.status != "OK":
    print("ERROR: step=smoke_verify_full_dryrun reason=verify_full_error")
    state.stop = true
  else:
    print("OK: step=smoke_verify_full_dryrun")

step = "health_summary"
if state.stop:
  print("SKIP: step=health_summary reason=STOP")
else:
  result = run("go run ./cmd/runner_health")
  # SOT: out/health.status
  print("OK: step=health_summary")

finally:
  if state.stop:
    print("STATUS: ERROR")
  else:
    print("STATUS: OK")
```

## SOT ファイル一覧

| ファイル | 生成元 | 判定値 |
|---|---|---|
| `out/verify-lite.status` | `cmd/verify-lite` / `cmd/verify_lite_host` | `status=OK\|ERROR\|SKIP` |
| `out/verify-full.status` | `cmd/verify-full` / `cmd/verify_full_host` | `status=OK\|ERROR\|SKIP` |
| `out/health.status` | `cmd/runner_health` | `status=OK\|ERROR\|SKIP` |
| `out/runner-setup.status` | `cmd/runner_setup` | `status=OK\|ERROR\|SKIP` |
| `.local/ci/state.json` | `cmd/ci_orch` | `stop=true\|false` |
