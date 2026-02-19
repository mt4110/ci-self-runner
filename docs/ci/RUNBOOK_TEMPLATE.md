# RUNBOOK TEMPLATE

## Purpose

- Centralize incident response, recovery procedures, and decision rationale in one place.
- Notifications belong in `#ci-alerts`; this is a “read / reference” artifact.

## Operating Rules

- Write only truth. Keep it short.
- Do not rely on exit codes to control flow; represent outcomes as `OK/SKIP/ERROR`.
- Always write in order: **Observation → Action → Result**.
- Include **one-line reason** for SKIP/ERROR.
- Never paste secrets (webhook URLs, tokens, PATs, private URLs). If leaked, rotate and record.

## Index Keywords (search)

- `[RUNNER]` self-hosted runner / registration / service
- `[COLIMA]` colima start/stop / cpu/mem/disk / mount-type
- `[DOCKER]` build cache / images / volumes
- `[DISK]` disk full / cleanup / GC
- `[DISCORD]` webhook rotate / notify failures
- `[GITHUB]` workflow / artifacts / run_id / gh secret
- `[SSH]` remote verify / rsync / keys

---

## Incident Entry Template (1 incident = 1 entry)

**[DATE: YYYY-MM-DD HH:MM JST] [TAG: RUNNER|COLIMA|DOCKER|DISK|DISCORD|GITHUB|SSH]**

### SYMPTOM

- (What happened? 1 line)

### OBSERVATION

- OK: ...
- SKIP: ... (one-line reason)
- ERROR: ... (one-line reason)

### ACTION

- (What you did — concise bullet list)

### RESULT

- OK: ...
- ERROR: ... (remaining issue, if any — one line)

### FOLLOW-UP

- (Prevent recurrence / next step — one line)
