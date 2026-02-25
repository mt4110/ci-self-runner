#!/usr/bin/env bash
set -euo pipefail

resolve_script_dir() {
  local src="${BASH_SOURCE[0]}"
  while [ -h "$src" ]; do
    local dir
    dir="$(cd -P "$(dirname "$src")" && pwd)"
    src="$(readlink "$src")"
    [[ "$src" != /* ]] && src="$dir/$src"
  done
  cd -P "$(dirname "$src")" && pwd
}

SCRIPT_DIR="$(resolve_script_dir)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

usage() {
  cat <<'USAGE'
Usage:
  ci-self <command> [options]

Commands:
  register   One-command runner registration for current repo
  run-watch  One-command verify workflow dispatch + watch
  run-focus  run-watch + All Green check + PR template sync
  watch      Watch latest verify workflow run
  help       Show this help

Examples:
  cd ~/dev/maakie-brainlab
  ci-self register
  ci-self run-watch
  ci-self run-focus
USAGE
}

resolve_repo() {
  local repo="${1:-}"
  if [[ -n "$repo" ]]; then
    printf '%s\n' "$repo"
    return
  fi
  gh repo view --json nameWithOwner --jq .nameWithOwner
}

current_branch() {
  git branch --show-current
}

resolve_pr_number() {
  local repo="$1"
  local branch="$2"
  gh pr list -R "$repo" --state open --head "$branch" --json number --jq '.[0].number // empty'
}

find_pr_template() {
  local root="${1:-$PWD}"
  local f=""
  for p in \
    ".github/pull_request_template.md" \
    ".github/PULL_REQUEST_TEMPLATE.md" \
    "PULL_REQUEST_TEMPLATE.md" \
    "docs/pull_request_template.md"; do
    if [[ -f "$root/$p" ]]; then
      echo "$root/$p"
      return 0
    fi
  done

  if [[ -d "$root/.github/PULL_REQUEST_TEMPLATE" ]]; then
    f="$(find "$root/.github/PULL_REQUEST_TEMPLATE" -maxdepth 1 -type f -name '*.md' | sort | head -n 1)"
    if [[ -n "$f" ]]; then
      echo "$f"
      return 0
    fi
  fi
  return 1
}

extract_title_from_template() {
  local file="$1"
  local title=""
  title="$(awk 'BEGIN{IGNORECASE=1} /^title[[:space:]]*:/ {sub(/^title[[:space:]]*:[[:space:]]*/,""); print; exit}' "$file")"
  if [[ -z "$title" ]]; then
    title="$(awk '/^#[[:space:]]+/ {sub(/^#[[:space:]]+/,""); print; exit}' "$file")"
  fi
  if [[ -z "$title" ]]; then
    title="$(awk 'NF {print; exit}' "$file")"
  fi
  printf '%s\n' "$title"
}

sync_pr_from_template() {
  local repo="$1"
  local pr_number="$2"
  local root="${3:-$PWD}"
  local tmpl=""
  if ! tmpl="$(find_pr_template "$root")"; then
    echo "SKIP: pr_template_sync reason=template_not_found"
    return 0
  fi

  local title
  title="$(extract_title_from_template "$tmpl")"
  if [[ -z "$title" ]]; then
    echo "SKIP: pr_template_sync reason=empty_title"
    return 0
  fi

  local body_file
  body_file="$(mktemp)"
  cp "$tmpl" "$body_file"
  gh pr edit "$pr_number" -R "$repo" --title "$title" --body-file "$body_file"
  rm -f "$body_file"
  echo "OK: pr_template_sync pr=$pr_number template=$tmpl title=$title"
}

cmd_register() {
  local repo=""
  local repo_dir="$PWD"
  local ref="main"
  local labels="self-hosted,mac-mini,colima,verify-full"
  local runner_name=""
  local runner_group="Default"
  local discord_webhook_url=""
  local force_workflow=0
  local skip_workflow=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo) repo="${2:-}"; shift 2 ;;
      --repo-dir) repo_dir="${2:-}"; shift 2 ;;
      --ref) ref="${2:-}"; shift 2 ;;
      --labels) labels="${2:-}"; shift 2 ;;
      --runner-name) runner_name="${2:-}"; shift 2 ;;
      --runner-group) runner_group="${2:-}"; shift 2 ;;
      --discord-webhook-url) discord_webhook_url="${2:-}"; shift 2 ;;
      --force-workflow) force_workflow=1; shift ;;
      --skip-workflow) skip_workflow=1; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self register [--repo owner/repo] [--repo-dir path] [--labels csv] [--runner-name name] [--runner-group name] [--discord-webhook-url url] [--force-workflow] [--skip-workflow]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for register: $1" >&2
        return 2
        ;;
    esac
  done

  repo="$(resolve_repo "$repo")"
  local args=(--repo "$repo" --repo-dir "$repo_dir" --ref "$ref" --labels "$labels" --runner-group "$runner_group" --skip-dispatch)
  [[ -n "$runner_name" ]] && args+=(--runner-name "$runner_name")
  [[ -n "$discord_webhook_url" ]] && args+=(--discord-webhook-url "$discord_webhook_url")
  [[ "$force_workflow" -eq 1 ]] && args+=(--force-workflow)
  [[ "$skip_workflow" -eq 1 ]] && args+=(--skip-workflow)

  bash "$ROOT_DIR/ops/ci/onboard_and_verify.sh" "${args[@]}"
}

cmd_run_watch() {
  local repo=""
  local ref="main"
  local sync_pr_template=0
  local pr_number=""
  local all_green=0
  local project_dir="$PWD"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo) repo="${2:-}"; shift 2 ;;
      --ref) ref="${2:-}"; shift 2 ;;
      --sync-pr-template) sync_pr_template=1; shift ;;
      --all-green) all_green=1; shift ;;
      --pr) pr_number="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self run-watch [--repo owner/repo] [--ref branch] [--all-green] [--sync-pr-template] [--pr <number>] [--project-dir <path>]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for run-watch: $1" >&2
        return 2
        ;;
    esac
  done
  repo="$(resolve_repo "$repo")"
  gh workflow run verify.yml --ref "$ref" -R "$repo"
  local run_id
  run_id="$(gh run list --workflow verify.yml -R "$repo" --limit 1 --json databaseId --jq '.[0].databaseId')"
  [[ -n "$run_id" ]] || { echo "ERROR: failed to resolve verify run id" >&2; return 1; }
  gh run watch "$run_id" -R "$repo" --exit-status

  if [[ "$all_green" -eq 1 || "$sync_pr_template" -eq 1 ]]; then
    local branch=""
    branch="$(current_branch 2>/dev/null || true)"
    if [[ -z "$pr_number" && -n "$branch" ]]; then
      pr_number="$(resolve_pr_number "$repo" "$branch")"
    fi
    if [[ -z "$pr_number" ]]; then
      echo "SKIP: pr_checks reason=pr_not_found_for_branch"
    else
      gh pr checks "$pr_number" -R "$repo" --watch
      if [[ "$sync_pr_template" -eq 1 ]]; then
        sync_pr_from_template "$repo" "$pr_number" "$project_dir"
      fi
    fi
  fi
}

cmd_watch() {
  local repo=""
  local run_id=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo) repo="${2:-}"; shift 2 ;;
      --run-id) run_id="${2:-}"; shift 2 ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self watch [--repo owner/repo] [--run-id id]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for watch: $1" >&2
        return 2
        ;;
    esac
  done
  repo="$(resolve_repo "$repo")"
  if [[ -z "$run_id" ]]; then
    run_id="$(gh run list --workflow verify.yml -R "$repo" --limit 1 --json databaseId --jq '.[0].databaseId')"
  fi
  [[ -n "$run_id" ]] || { echo "ERROR: failed to resolve verify run id" >&2; return 1; }
  gh run watch "$run_id" -R "$repo" --exit-status
}

main() {
  local cmd="${1:-help}"
  shift || true
  case "$cmd" in
    register) cmd_register "$@" ;;
    run-watch) cmd_run_watch "$@" ;;
    run-focus) cmd_run_watch --all-green --sync-pr-template "$@" ;;
    watch) cmd_watch "$@" ;;
    help|-h|--help) usage ;;
    *)
      echo "ERROR: unknown command: $cmd" >&2
      usage >&2
      return 2
      ;;
  esac
}

main "$@"
