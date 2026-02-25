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
  watch      Watch latest verify workflow run
  help       Show this help

Examples:
  cd ~/dev/maakie-brainlab
  ci-self register
  ci-self run-watch
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
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo) repo="${2:-}"; shift 2 ;;
      --ref) ref="${2:-}"; shift 2 ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self run-watch [--repo owner/repo] [--ref branch]
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
