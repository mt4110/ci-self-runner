#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  onboard_and_verify.sh --repo <owner/repo> [options]

Options:
  --repo <owner/repo>     Target repository (required)
  --repo-dir <path>       Local path of target repo (optional; used to scaffold verify.yml/pr template)
  --ref <branch>          Branch/ref for workflow dispatch (default: main)
  --labels <csv>          Runner labels for registration (default: self-hosted,mac-mini,colima,verify-full)
  --runner-name <name>    Runner name override
  --runner-group <name>   Runner group (default: Default)
  --discord-webhook-url <url>
                          Set DISCORD_WEBHOOK_URL secret in target repo
  --force-workflow        Overwrite existing verify.yml when scaffolding
  --skip-workflow         Do not scaffold verify.yml to --repo-dir
  --skip-dispatch         Do not run gh workflow run verify.yml
  -h, --help              Show this help

Examples:
  # 最短: runner登録 + owner変数 + verify dispatch
  bash ops/ci/onboard_and_verify.sh --repo mt4110/maakie-brainlab

  # verify.yml / PR template をローカル作成してから dispatch
  bash ops/ci/onboard_and_verify.sh --repo mt4110/maakie-brainlab --repo-dir ~/dev/maakie-brainlab
USAGE
}

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

run_go() {
  if command -v go >/dev/null 2>&1; then
    go "$@"
    return
  fi
  if command -v mise >/dev/null 2>&1; then
    mise x -- go "$@"
    return
  fi
  echo "ERROR: go not found (install go or mise)" >&2
  exit 1
}

REPO=""
REPO_DIR=""
REF="main"
LABELS="self-hosted,mac-mini,colima,verify-full"
RUNNER_NAME=""
RUNNER_GROUP="Default"
DISCORD_WEBHOOK_URL=""
FORCE_WORKFLOW=0
SKIP_WORKFLOW=0
SKIP_DISPATCH=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --repo-dir)
      REPO_DIR="${2:-}"
      shift 2
      ;;
    --ref)
      REF="${2:-}"
      shift 2
      ;;
    --labels)
      LABELS="${2:-}"
      shift 2
      ;;
    --runner-name)
      RUNNER_NAME="${2:-}"
      shift 2
      ;;
    --runner-group)
      RUNNER_GROUP="${2:-}"
      shift 2
      ;;
    --discord-webhook-url)
      DISCORD_WEBHOOK_URL="${2:-}"
      shift 2
      ;;
    --force-workflow)
      FORCE_WORKFLOW=1
      shift
      ;;
    --skip-workflow)
      SKIP_WORKFLOW=1
      shift
      ;;
    --skip-dispatch)
      SKIP_DISPATCH=1
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

if [[ -z "$REPO" ]]; then
  echo "ERROR: --repo is required" >&2
  usage >&2
  exit 2
fi

echo "OK: target_repo=$REPO ref=$REF"

if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: gh command not found" >&2
  exit 1
fi

echo "OK: ensure_colima_running"
colima status >/dev/null 2>&1 || colima start

echo "OK: runner_setup_start"
runner_setup_args=(--apply --repo "$REPO" --labels "$LABELS" --runner-group "$RUNNER_GROUP")
if [[ -n "$RUNNER_NAME" ]]; then
  runner_setup_args+=(--name "$RUNNER_NAME")
fi
run_go run ./cmd/runner_setup "${runner_setup_args[@]}"

echo "OK: runner_health_start"
run_go run ./cmd/runner_health

OWNER="$(gh repo view "$REPO" --json owner --jq .owner.login)"
echo "OK: set_variable SELF_HOSTED_OWNER=$OWNER"
gh variable set SELF_HOSTED_OWNER -b "$OWNER" -R "$REPO"

if [[ -n "$DISCORD_WEBHOOK_URL" ]]; then
  echo "OK: set_secret DISCORD_WEBHOOK_URL"
  printf '%s' "$DISCORD_WEBHOOK_URL" | gh secret set DISCORD_WEBHOOK_URL -R "$REPO"
fi

if [[ "$SKIP_WORKFLOW" -ne 1 && -n "$REPO_DIR" ]]; then
  echo "OK: scaffold_verify_workflow repo_dir=$REPO_DIR"
  scaffold_args=(--repo "$REPO_DIR" --apply)
  if [[ "$FORCE_WORKFLOW" -eq 1 ]]; then
    scaffold_args+=(--force)
  fi
  bash ops/ci/scaffold_verify_workflow.sh "${scaffold_args[@]}"
fi

if [[ -n "$REPO_DIR" ]]; then
  echo "OK: scaffold_pr_template repo_dir=$REPO_DIR"
  bash ops/ci/scaffold_pr_template.sh --repo "$REPO_DIR" --apply
  echo "NOTE: commit workflow/template changes in $REPO_DIR if needed (.github/workflows/verify.yml/.github/pull_request_template.md/.gitignore)"
fi

if [[ "$SKIP_DISPATCH" -eq 1 ]]; then
  echo "SKIP: workflow_dispatch reason=skip_dispatch_flag"
  exit 0
fi

has_verify="$(
  gh api "repos/$REPO/actions/workflows" --jq '.workflows[].path' \
    | rg -x '\.github/workflows/verify\.yml' || true
)"
if [[ -z "$has_verify" ]]; then
  echo "ERROR: .github/workflows/verify.yml not found in remote repo ($REPO)" >&2
  echo "HINT: commit/push verify.yml first, or run with --repo-dir and commit result." >&2
  exit 1
fi

echo "OK: workflow_dispatch verify.yml ref=$REF"
gh workflow run verify.yml --ref "$REF" -R "$REPO"

RUN_ID="$(gh run list --workflow verify.yml -R "$REPO" --limit 1 --json databaseId --jq '.[0].databaseId')"
if [[ -z "$RUN_ID" ]]; then
  echo "ERROR: failed to resolve latest verify run id" >&2
  exit 1
fi

echo "OK: watch_run id=$RUN_ID"
gh run watch "$RUN_ID" -R "$REPO" --exit-status
