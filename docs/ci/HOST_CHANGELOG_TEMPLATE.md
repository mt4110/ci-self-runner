# HOST (Mac mini) CHANGELOG TEMPLATE

## Role
- Record host-specific change history, maintenance logs, and recovery traces for the Mac mini CI host.
- CI failure notifications belong in `#ci-alerts`.
- Procedure details belong in the RUNBOOK (use `RUNBOOK_TEMPLATE.md`).

## Rules
- 1 change = 1 entry (searchable, audit-friendly).
- Always include a one-line reason.
- Be explicit with numbers (cpu/mem/disk/ttl/keep).
- Always include a one-line rollback method.
- Never paste secrets (keys, webhook URLs, tokens, PATs).

---

## Change Entry Template (1 change = 1 entry)

**[DATE: YYYY-MM-DD HH:MM JST] [AREA: COLIMA|RUNNER|DOCKER|DISK|GC|SSH|OS]**

### CHANGE
- (What changed? 1 line)

### REASON
- (Why? 1 line)

### DETAILS
- (Settings / paths / key values — short)
  - example: colima cpu=__ mem=__ disk=__ mount=__
  - example: GC ttl_logs_days=__ keep_reviewpack=__ keep_gha=__

### VERIFY
- OK: ... (observation)
- SKIP: ... (one-line reason)
- ERROR: ... (one-line reason)

### ROLLBACK
- (How to revert — one line)
  - example: revert to previous colima profile __ / restore config from __

### NOTES
- (Insight / next concern — one line)
