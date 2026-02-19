# DISCORD_PLAN

## Pseudo Code

```text
state.stop = false
state.errors = []

try:
  step = "preflight_paths"
  if missing(required_paths):
    print("ERROR: step=preflight_paths reason=missing_path")
    state.stop = true
  else:
    print("OK: step=preflight_paths")

  step = "docs"
  if state.stop:
    print("SKIP: step=docs reason=STOP")
  else:
    write(SECRETS_POLICY.md)
    write(DISCORD_NOTIFICATIONS.md)
    print("OK: step=docs")

  step = "go_non_exit_and_scan"
  if state.stop:
    print("SKIP: step=go_non_exit_and_scan reason=STOP")
  else:
    update(verify_full_main)
    update(verify_lite_main)
    print("OK: step=go_non_exit_and_scan")

  step = "notify_tool"
  if state.stop:
    print("SKIP: step=notify_tool reason=STOP")
  else:
    add(cmd_notify_discord)
    print("OK: step=notify_tool")

  step = "workflow_notify"
  if state.stop:
    print("SKIP: step=workflow_notify reason=STOP")
  else:
    update(verify_workflow_failure_notification)
    print("OK: step=workflow_notify")

  step = "verify"
  if state.stop:
    print("SKIP: step=verify reason=STOP")
  else:
    run(go_test_all)
    run(review_pack_core)
    run(review_pack_optional)
    run(notify_discord_dry_run)
    print("OK: step=verify")

catch err:
  print("ERROR: step=catch reason=" + err)
  state.stop = true

if state.stop:
  print("ERROR: discord plan stopped")
else:
  print("OK: discord plan completed")
```
