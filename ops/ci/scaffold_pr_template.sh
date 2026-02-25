#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scaffold_pr_template.sh [--repo <path>] [--apply] [--force]

Options:
  --repo <path>  Target repository path (default: current directory)
  --apply        Write PR template to target repository
  --force        Overwrite existing template path
  -h, --help     Show this help

Behavior:
  - Detects common PR template paths.
  - If a template already exists, exits with SKIP unless --force is set.
  - Without --apply, prints template content to stdout.
USAGE
}

find_existing_template() {
  local root="$1"
  local p=""
  for p in \
    ".github/pull_request_template.md" \
    ".github/PULL_REQUEST_TEMPLATE.md" \
    "PULL_REQUEST_TEMPLATE.md" \
    "docs/pull_request_template.md"; do
    if [[ -f "$root/$p" ]]; then
      printf '%s\n' "$root/$p"
      return 0
    fi
  done
  if [[ -d "$root/.github/PULL_REQUEST_TEMPLATE" ]]; then
    p="$(find "$root/.github/PULL_REQUEST_TEMPLATE" -maxdepth 1 -type f -name '*.md' | sort | head -n 1 || true)"
    if [[ -n "$p" ]]; then
      printf '%s\n' "$p"
      return 0
    fi
  fi
  return 1
}

render_template() {
  cat <<'MD'
## Summary
- 

## Why
- 

## Changes
- 

## Verification
- [ ] `ci-self run-focus` passed
- [ ] Logs/status files checked (`out/*.status`)

## Risks
- [ ] Rollback path considered
MD
}

REPO_DIR="."
APPLY=0
FORCE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO_DIR="${2:-}"
      shift 2
      ;;
    --apply)
      APPLY=1
      shift
      ;;
    --force)
      FORCE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "ERROR: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "$REPO_DIR" ]]; then
  echo "ERROR: repo directory not found: $REPO_DIR" >&2
  exit 2
fi

REPO_DIR="$(cd "$REPO_DIR" && pwd)"
existing_template="$(find_existing_template "$REPO_DIR" || true)"
target_template="$REPO_DIR/.github/pull_request_template.md"

if [[ -n "$existing_template" ]]; then
  target_template="$existing_template"
fi

if [[ "$APPLY" -eq 0 ]]; then
  render_template
  exit 0
fi

if [[ -n "$existing_template" && "$FORCE" -ne 1 ]]; then
  echo "SKIP: pr template already exists: $existing_template (use --force to overwrite)"
  exit 0
fi

mkdir -p "$(dirname "$target_template")"
render_template >"$target_template"
echo "OK: wrote $target_template"

