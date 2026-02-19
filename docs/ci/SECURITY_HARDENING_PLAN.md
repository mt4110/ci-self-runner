# SECURITY_HARDENING_PLAN

## 目的

self-hosted runner を「漏えいしにくい・横展開しにくい・住み着かれにくい」状態に寄せる。  
前提: このリポは single-owner 運用（外部PRをself-hostedで実行しない）。

## PLAN（pseudo）

```text
STOP = false

if runner_user != "ci" (or dedicated user)
  print("ERROR: runner user is not isolated")
  STOP = true
else
  print("OK: runner user isolation")

for workflow in .github/workflows/*.yml
  if contains("pull_request_target")
    print("ERROR: forbidden event pull_request_target")
    STOP = true
    continue

  if contains("runs-on: self-hosted") and missing fork/owner guard
    print("ERROR: self-hosted job missing guard")
    STOP = true
    continue

  for each uses: line
    if action ref is not pinned to commit SHA
      print("ERROR: unpinned action ref")
      STOP = true
      continue

if STOP
  print("SKIP: hardening apply reason=preflight failed")
  goto END

try
  enforce secrets policy
  print("OK: secrets policy")
catch e
  print("ERROR: secrets policy " + e)
  STOP = true

try
  enforce container isolation
  print("OK: container isolation")
catch e
  print("ERROR: container isolation " + e)
  STOP = true

try
  define network posture (allowlist or monitor)
  print("OK: network posture")
catch e
  print("SKIP: network posture reason=" + e)

END:
print("OK/SKIP/ERROR summary")
return normally
```

## 完了条件

- workflow が SHA pin + self-hosted guard を満たす
- `verify-lite` が workflow policy scan を実行する
- README と RUNBOOK に single-owner と緊急停止を明記
