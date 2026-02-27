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
CONFIG_BASENAME=".ci-self.env"
CONFIG_FILE=""
CONFIG_REPO=""
CONFIG_REF=""
CONFIG_PROJECT_DIR=""
CONFIG_REMOTE_HOST=""
CONFIG_REMOTE_PROJECT_DIR=""
CONFIG_REMOTE_CLI=""
CONFIG_LABELS=""
CONFIG_RUNNER_NAME=""
CONFIG_RUNNER_GROUP=""
CONFIG_DISCORD_WEBHOOK_URL=""
CONFIG_FORCE_WORKFLOW=""
CONFIG_SKIP_WORKFLOW=""
CONFIG_PR_BASE=""

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s\n' "$s"
}

expand_local_path() {
  local p="$1"
  if [[ "$p" == "~/"* ]]; then
    printf '%s\n' "${HOME}/${p#~/}"
  else
    printf '%s\n' "$p"
  fi
}

run_go_cmd() {
  if command -v go >/dev/null 2>&1; then
    if go "$@"; then
      return 0
    fi
    echo "WARN: go command failed; retrying via mise" >&2
  fi
  if command -v mise >/dev/null 2>&1; then
    mise x -- go "$@"
    return $?
  fi
  return 127
}

unquote_value() {
  local v="$1"
  local n="${#v}"
  if (( n >= 2 )); then
    if [[ "${v:0:1}" == '"' && "${v:n-1:1}" == '"' ]]; then
      v="${v:1:n-2}"
    elif [[ "${v:0:1}" == "'" && "${v:n-1:1}" == "'" ]]; then
      v="${v:1:n-2}"
    fi
  fi
  printf '%s\n' "$v"
}

config_bool_to_int() {
  local v
  v="$(echo "${1:-}" | tr '[:upper:]' '[:lower:]')"
  case "$v" in
    1|true|yes|on) echo 1 ;;
    *) echo 0 ;;
  esac
}

load_config() {
  local search_root="$PWD"
  local git_root=""
  local f=""
  git_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
  if [[ -n "$git_root" && -f "$git_root/$CONFIG_BASENAME" ]]; then
    f="$git_root/$CONFIG_BASENAME"
  elif [[ -f "$search_root/$CONFIG_BASENAME" ]]; then
    f="$search_root/$CONFIG_BASENAME"
  else
    return 0
  fi

  CONFIG_FILE="$f"

  local raw line key val
  while IFS= read -r raw || [[ -n "$raw" ]]; do
    line="$(trim "$raw")"
    [[ -z "$line" || "${line:0:1}" == "#" ]] && continue
    [[ "$line" == export\ * ]] && line="$(trim "${line#export }")"
    [[ "$line" == *=* ]] || continue
    key="$(trim "${line%%=*}")"
    val="$(trim "${line#*=}")"
    val="$(unquote_value "$val")"
    case "$key" in
      CI_SELF_REPO) CONFIG_REPO="$val" ;;
      CI_SELF_REF) CONFIG_REF="$val" ;;
      CI_SELF_PROJECT_DIR) CONFIG_PROJECT_DIR="$val" ;;
      CI_SELF_REMOTE_HOST) CONFIG_REMOTE_HOST="$val" ;;
      CI_SELF_REMOTE_PROJECT_DIR) CONFIG_REMOTE_PROJECT_DIR="$val" ;;
      CI_SELF_REMOTE_CLI) CONFIG_REMOTE_CLI="$val" ;;
      CI_SELF_LABELS) CONFIG_LABELS="$val" ;;
      CI_SELF_RUNNER_NAME) CONFIG_RUNNER_NAME="$val" ;;
      CI_SELF_RUNNER_GROUP) CONFIG_RUNNER_GROUP="$val" ;;
      CI_SELF_DISCORD_WEBHOOK_URL) CONFIG_DISCORD_WEBHOOK_URL="$val" ;;
      CI_SELF_FORCE_WORKFLOW) CONFIG_FORCE_WORKFLOW="$val" ;;
      CI_SELF_SKIP_WORKFLOW) CONFIG_SKIP_WORKFLOW="$val" ;;
      CI_SELF_PR_BASE) CONFIG_PR_BASE="$val" ;;
      *) ;;
    esac
  done < "$f"
}

usage() {
  cat <<'USAGE'
Usage:
  ci-self <command> [options]

Commands:
  up         One-command local register + run-focus
  focus      run-focus + optional PR auto-create
  doctor     Dependency/runner checks (with optional --fix)
  config-init  Create .ci-self.env template in current project
  register   One-command runner registration for current repo
  run-watch  One-command verify workflow dispatch + watch
  run-focus  run-watch + All Green check + PR template sync
  remote-ci        Key-only SSH + sync + remote verify + fetch results
  remote-register  Run `register` over SSH on remote host
  remote-run-focus Run `run-focus` over SSH on remote host
  remote-up        Run `remote-register` + `remote-run-focus` in one command
  watch      Watch latest verify workflow run
  help       Show this help

Examples:
  cd ~/dev/maakie-brainlab
  ci-self up
  ci-self focus
  ci-self doctor --fix
  ci-self config-init
  ci-self register
  ci-self run-watch
  ci-self run-focus
  ci-self remote-ci --host <user>@<mac-mini> --project-dir '~/dev/maakie-brainlab'
  ci-self remote-up --host mac-mini.local --project-dir ~/dev/maakie-brainlab
USAGE
}

resolve_repo() {
  local repo="${1:-}"
  if [[ -n "$repo" ]]; then
    printf '%s\n' "$repo"
    return
  fi
  if [[ -n "$CONFIG_REPO" ]]; then
    printf '%s\n' "$CONFIG_REPO"
    return
  fi
  gh repo view --json nameWithOwner --jq .nameWithOwner
}

resolve_ref() {
  local ref="${1:-}"
  if [[ -n "$ref" ]]; then
    printf '%s\n' "$ref"
    return
  fi
  if [[ -n "$CONFIG_REF" ]]; then
    printf '%s\n' "$CONFIG_REF"
    return
  fi
  echo "main"
}

current_branch() {
  git branch --show-current
}

ensure_verify_workflow_nix_compat() {
  local project_dir="${1:-}"
  [[ -z "$project_dir" ]] && return 0
  [[ -d "$project_dir" ]] || return 0

  if [[ ! -f "$project_dir/flake.nix" ]]; then
    return 0
  fi

  local wf="$project_dir/.github/workflows/verify.yml"
  if [[ ! -f "$wf" ]]; then
    echo "ERROR: verify workflow not found for flake repo: $wf" >&2
    echo "HINT: bash $ROOT_DIR/ops/ci/scaffold_verify_workflow.sh --repo $project_dir --apply" >&2
    return 1
  fi

  if ! grep -Fq "nix-daemon.sh" "$wf"; then
    echo "ERROR: verify.yml is outdated for nix runner env: $wf" >&2
    echo "HINT: bash $ROOT_DIR/ops/ci/scaffold_verify_workflow.sh --repo $project_dir --apply --force" >&2
    echo "HINT: commit/push updated verify.yml, then rerun ci-self" >&2
    return 1
  fi
}

resolve_verify_workflow_id() {
  local repo="$1"
  local workflows=""
  local id=""
  local path=""
  local name=""
  local lc_path=""
  local lc_name=""
  local verify_yaml_id=""
  local verify_name_id=""
  local verify_path_id=""

  workflows="$(gh api "repos/$repo/actions/workflows" --jq '.workflows[]? | [.id, (.path // ""), (.name // "")] | @tsv')" || return 1

  while IFS=$'\t' read -r id path name; do
    [[ -n "$id" ]] || continue
    lc_path="$(printf '%s' "$path" | tr '[:upper:]' '[:lower:]')"
    lc_name="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')"

    if [[ "$lc_path" == ".github/workflows/verify.yml" ]]; then
      printf '%s\n' "$id"
      return 0
    fi
    [[ -z "$verify_yaml_id" && "$lc_path" == ".github/workflows/verify.yaml" ]] && verify_yaml_id="$id"
    [[ -z "$verify_name_id" && "$lc_name" == "verify" ]] && verify_name_id="$id"
    [[ -z "$verify_path_id" && "$lc_path" == *"/verify."* ]] && verify_path_id="$id"
  done <<< "$workflows"

  if [[ -n "$verify_yaml_id" ]]; then
    printf '%s\n' "$verify_yaml_id"
    return 0
  fi
  if [[ -n "$verify_name_id" ]]; then
    printf '%s\n' "$verify_name_id"
    return 0
  fi
  if [[ -n "$verify_path_id" ]]; then
    printf '%s\n' "$verify_path_id"
    return 0
  fi
}

print_verify_workflow_missing_hint() {
  local repo="$1"
  local project_dir="${2:-}"
  echo "ERROR: verify workflow not found in remote repo ($repo)" >&2
  echo "HINT: expected .github/workflows/verify.yml (or verify.yaml) in $repo" >&2
  if [[ -n "$project_dir" && -d "$project_dir" ]]; then
    if [[ -f "$project_dir/.github/workflows/verify.yml" || -f "$project_dir/.github/workflows/verify.yaml" ]]; then
      echo "HINT: local workflow exists in $project_dir/.github/workflows; commit/push then rerun ci-self" >&2
    else
      echo "HINT: bash $ROOT_DIR/ops/ci/scaffold_verify_workflow.sh --repo $project_dir --apply" >&2
      echo "HINT: commit/push generated .github/workflows/verify.yml, then rerun ci-self" >&2
    fi
  fi
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

ensure_branch_pushed() {
  local branch="$1"
  [[ -n "$branch" ]] || return 0
  if [[ "$branch" == "main" ]]; then
    echo "SKIP: branch_push reason=main_branch"
    return 0
  fi
  if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "ERROR: working tree is dirty; commit or stash before focus auto-push" >&2
    return 1
  fi
  if git ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
    git push origin "$branch"
  else
    git push -u origin "$branch"
  fi
}

create_pr_if_missing() {
  local repo="$1"
  local branch="$2"
  local base="$3"
  local root="${4:-$PWD}"
  local pr_number=""

  pr_number="$(resolve_pr_number "$repo" "$branch")"
  if [[ -n "$pr_number" ]]; then
    echo "$pr_number"
    return 0
  fi

  local pr_title=""
  local body_file=""
  local tmpl=""
  if tmpl="$(find_pr_template "$root")"; then
    pr_title="$(extract_title_from_template "$tmpl")"
    if [[ -n "$pr_title" ]]; then
      body_file="$(mktemp)"
      cp "$tmpl" "$body_file"
    fi
  fi

  local out=""
  if [[ -n "$body_file" ]]; then
    out="$(gh pr create -R "$repo" --base "$base" --head "$branch" --title "$pr_title" --body-file "$body_file")"
    rm -f "$body_file"
  else
    out="$(gh pr create -R "$repo" --base "$base" --head "$branch" --fill)"
  fi
  echo "OK: pr_created $out"

  pr_number="$(resolve_pr_number "$repo" "$branch")"
  if [[ -z "$pr_number" ]]; then
    echo "ERROR: PR creation output received but PR number could not be resolved" >&2
    return 1
  fi
  echo "$pr_number"
}

cmd_register() {
  local repo=""
  local repo_dir="$PWD"
  local ref=""
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

  [[ -z "$repo_dir" ]] && repo_dir="$PWD"
  [[ -n "$CONFIG_PROJECT_DIR" && "$repo_dir" == "$PWD" ]] && repo_dir="$CONFIG_PROJECT_DIR"
  repo_dir="$(expand_local_path "$repo_dir")"
  [[ -z "$ref" ]] && ref="$(resolve_ref "$ref")"
  [[ -n "$CONFIG_LABELS" && "$labels" == "self-hosted,mac-mini,colima,verify-full" ]] && labels="$CONFIG_LABELS"
  [[ -z "$runner_name" && -n "$CONFIG_RUNNER_NAME" ]] && runner_name="$CONFIG_RUNNER_NAME"
  [[ "$runner_group" == "Default" && -n "$CONFIG_RUNNER_GROUP" ]] && runner_group="$CONFIG_RUNNER_GROUP"
  [[ -z "$discord_webhook_url" && -n "$CONFIG_DISCORD_WEBHOOK_URL" ]] && discord_webhook_url="$CONFIG_DISCORD_WEBHOOK_URL"
  [[ "$force_workflow" -eq 0 ]] && force_workflow="$(config_bool_to_int "$CONFIG_FORCE_WORKFLOW")"
  [[ "$skip_workflow" -eq 0 ]] && skip_workflow="$(config_bool_to_int "$CONFIG_SKIP_WORKFLOW")"

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
  local ref=""
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
  [[ -z "$project_dir" ]] && project_dir="$PWD"
  [[ -n "$CONFIG_PROJECT_DIR" && "$project_dir" == "$PWD" ]] && project_dir="$CONFIG_PROJECT_DIR"
  project_dir="$(expand_local_path "$project_dir")"
  [[ -z "$ref" ]] && ref="$(resolve_ref "$ref")"
  repo="$(resolve_repo "$repo")"
  ensure_verify_workflow_nix_compat "$project_dir"

  local workflow_id=""
  workflow_id="$(resolve_verify_workflow_id "$repo")"
  if [[ -z "$workflow_id" ]]; then
    print_verify_workflow_missing_hint "$repo" "$project_dir"
    return 1
  fi

  gh workflow run "$workflow_id" --ref "$ref" -R "$repo"
  local run_id
  run_id="$(gh run list --workflow "$workflow_id" -R "$repo" --limit 1 --json databaseId --jq '.[0].databaseId')"
  [[ -n "$run_id" ]] || { echo "ERROR: failed to resolve verify run id (workflow=$workflow_id)" >&2; return 1; }
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
    local workflow_id=""
    workflow_id="$(resolve_verify_workflow_id "$repo")"
    if [[ -z "$workflow_id" ]]; then
      print_verify_workflow_missing_hint "$repo"
      return 1
    fi
    run_id="$(gh run list --workflow "$workflow_id" -R "$repo" --limit 1 --json databaseId --jq '.[0].databaseId')"
  fi
  [[ -n "$run_id" ]] || { echo "ERROR: failed to resolve verify run id" >&2; return 1; }
  gh run watch "$run_id" -R "$repo" --exit-status
}

cmd_up() {
  local repo=""
  local repo_dir="$PWD"
  local ref=""
  local labels=""
  local runner_name=""
  local runner_group=""
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
Usage: ci-self up [--repo owner/repo] [--repo-dir path] [--ref branch] [--labels csv]
                  [--runner-name name] [--runner-group name] [--discord-webhook-url url]
                  [--force-workflow] [--skip-workflow]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for up: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$repo_dir" ]] && repo_dir="$PWD"
  [[ -n "$CONFIG_PROJECT_DIR" && "$repo_dir" == "$PWD" ]] && repo_dir="$CONFIG_PROJECT_DIR"
  repo_dir="$(expand_local_path "$repo_dir")"
  [[ -z "$ref" ]] && ref="$(resolve_ref "$ref")"
  [[ -z "$labels" && -n "$CONFIG_LABELS" ]] && labels="$CONFIG_LABELS"
  [[ -z "$runner_name" && -n "$CONFIG_RUNNER_NAME" ]] && runner_name="$CONFIG_RUNNER_NAME"
  [[ -z "$runner_group" && -n "$CONFIG_RUNNER_GROUP" ]] && runner_group="$CONFIG_RUNNER_GROUP"
  [[ -z "$discord_webhook_url" && -n "$CONFIG_DISCORD_WEBHOOK_URL" ]] && discord_webhook_url="$CONFIG_DISCORD_WEBHOOK_URL"
  [[ "$force_workflow" -eq 0 ]] && force_workflow="$(config_bool_to_int "$CONFIG_FORCE_WORKFLOW")"
  [[ "$skip_workflow" -eq 0 ]] && skip_workflow="$(config_bool_to_int "$CONFIG_SKIP_WORKFLOW")"

  local register_args=(--repo-dir "$repo_dir" --ref "$ref")
  [[ -n "$repo" ]] && register_args+=(--repo "$repo")
  [[ -n "$labels" ]] && register_args+=(--labels "$labels")
  [[ -n "$runner_name" ]] && register_args+=(--runner-name "$runner_name")
  [[ -n "$runner_group" ]] && register_args+=(--runner-group "$runner_group")
  [[ -n "$discord_webhook_url" ]] && register_args+=(--discord-webhook-url "$discord_webhook_url")
  [[ "$force_workflow" -eq 1 ]] && register_args+=(--force-workflow)
  [[ "$skip_workflow" -eq 1 ]] && register_args+=(--skip-workflow)
  cmd_register "${register_args[@]}"

  local run_focus_args=(--ref "$ref" --project-dir "$repo_dir")
  [[ -n "$repo" ]] && run_focus_args+=(--repo "$repo")
  cmd_run_watch --all-green --sync-pr-template "${run_focus_args[@]}"
}

cmd_focus() {
  local repo=""
  local ref=""
  local base="main"
  local project_dir="$PWD"
  local push_branch=1
  local create_pr=1
  local pr_number=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo) repo="${2:-}"; shift 2 ;;
      --ref) ref="${2:-}"; shift 2 ;;
      --base) base="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      --pr) pr_number="${2:-}"; shift 2 ;;
      --no-push) push_branch=0; shift ;;
      --no-pr-create) create_pr=0; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self focus [--repo owner/repo] [--ref branch] [--base main] [--project-dir path] [--pr number] [--no-push] [--no-pr-create]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for focus: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$project_dir" ]] && project_dir="$PWD"
  [[ -n "$CONFIG_PROJECT_DIR" && "$project_dir" == "$PWD" ]] && project_dir="$CONFIG_PROJECT_DIR"
  project_dir="$(expand_local_path "$project_dir")"
  [[ -z "$base" && -n "$CONFIG_PR_BASE" ]] && base="$CONFIG_PR_BASE"
  [[ -n "$CONFIG_PR_BASE" && "$base" == "main" ]] && base="$CONFIG_PR_BASE"

  local branch
  branch="$(current_branch 2>/dev/null || true)"
  [[ -z "$ref" && -n "$branch" ]] && ref="$branch"
  [[ -z "$ref" ]] && ref="$(resolve_ref "$ref")"
  repo="$(resolve_repo "$repo")"

  if [[ -n "$branch" && "$push_branch" -eq 1 ]]; then
    ensure_branch_pushed "$branch"
  fi

  if [[ -z "$pr_number" && -n "$branch" ]]; then
    pr_number="$(resolve_pr_number "$repo" "$branch")"
  fi

  local run_watch_args=(--repo "$repo" --ref "$ref" --all-green --sync-pr-template --project-dir "$project_dir")
  [[ -n "$pr_number" ]] && run_watch_args+=(--pr "$pr_number")
  cmd_run_watch "${run_watch_args[@]}"

  if [[ "$create_pr" -ne 1 ]]; then
    echo "SKIP: pr_create reason=no_pr_create_flag"
    return 0
  fi
  if [[ -z "$branch" || "$branch" == "$base" ]]; then
    echo "SKIP: pr_create reason=invalid_branch branch=$branch base=$base"
    return 0
  fi

  if [[ -z "$pr_number" ]]; then
    local pr_out
    pr_out="$(create_pr_if_missing "$repo" "$branch" "$base" "$project_dir")"
    pr_number="$(printf '%s\n' "$pr_out" | tail -n 1)"
  fi
  [[ -n "$pr_number" ]] || { echo "ERROR: failed to resolve PR number in focus" >&2; return 1; }

  gh pr checks "$pr_number" -R "$repo" --watch
  sync_pr_from_template "$repo" "$pr_number" "$project_dir"
}

cmd_doctor() {
  local repo=""
  local repo_dir="$PWD"
  local fix=0
  local verbose=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo) repo="${2:-}"; shift 2 ;;
      --repo-dir) repo_dir="${2:-}"; shift 2 ;;
      --fix) fix=1; shift ;;
      --verbose) verbose=1; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self doctor [--repo owner/repo] [--repo-dir path] [--fix] [--verbose]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for doctor: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$repo_dir" ]] && repo_dir="$PWD"
  [[ -n "$CONFIG_PROJECT_DIR" && "$repo_dir" == "$PWD" ]] && repo_dir="$CONFIG_PROJECT_DIR"
  repo_dir="$(expand_local_path "$repo_dir")"

  local failed=0
  local item=""
  for item in gh colima docker; do
    if command -v "$item" >/dev/null 2>&1; then
      echo "OK: doctor check=$item reason=available"
    else
      echo "ERROR: doctor check=$item reason=missing"
      failed=1
    fi
  done

  if command -v gh >/dev/null 2>&1; then
    if gh auth status >/dev/null 2>&1; then
      echo "OK: doctor check=gh_auth reason=authenticated"
    else
      echo "ERROR: doctor check=gh_auth reason=not_authenticated"
      echo "HINT: run 'gh auth login'"
      failed=1
    fi
  fi

  if command -v colima >/dev/null 2>&1; then
    if colima status >/dev/null 2>&1; then
      echo "OK: doctor check=colima reason=running"
    else
      if [[ "$fix" -eq 1 ]]; then
        echo "OK: doctor fix=colima_start"
        colima start
        if colima status >/dev/null 2>&1; then
          echo "OK: doctor check=colima reason=running_after_fix"
        else
          echo "ERROR: doctor check=colima reason=still_not_running"
          failed=1
        fi
      else
        echo "ERROR: doctor check=colima reason=not_running"
        failed=1
      fi
    fi
  fi

  if command -v docker >/dev/null 2>&1; then
    if docker info >/dev/null 2>&1; then
      echo "OK: doctor check=docker reason=available"
    else
      echo "ERROR: doctor check=docker reason=daemon_unreachable"
      failed=1
    fi
  fi

  repo="$(resolve_repo "$repo" 2>/dev/null || true)"
  if [[ -n "$repo" ]] && command -v gh >/dev/null 2>&1; then
    if [[ "$fix" -eq 1 ]]; then
      local owner
      owner="$(gh repo view "$repo" --json owner --jq .owner.login 2>/dev/null || true)"
      if [[ -n "$owner" ]]; then
        gh variable set SELF_HOSTED_OWNER -b "$owner" -R "$repo" >/dev/null 2>&1 || true
        echo "OK: doctor fix=set_owner_variable repo=$repo owner=$owner"
      fi
    fi

    local online_count
    online_count="$(gh api "repos/$repo/actions/runners" --jq '[.runners[] | select(.status=="online")] | length' 2>/dev/null || echo "0")"
    if [[ "$online_count" -gt 0 ]]; then
      echo "OK: doctor check=runner_online repo=$repo count=$online_count"
    else
      if [[ "$fix" -eq 1 ]]; then
        echo "OK: doctor fix=runner_setup repo=$repo"
        if ! run_go_cmd run ./cmd/runner_setup --apply --repo "$repo"; then
          echo "ERROR: doctor fix=runner_setup reason=runner_setup_failed"
          failed=1
        fi
        online_count="$(gh api "repos/$repo/actions/runners" --jq '[.runners[] | select(.status=="online")] | length' 2>/dev/null || echo "0")"
        if [[ "$online_count" -gt 0 ]]; then
          echo "OK: doctor check=runner_online reason=online_after_fix repo=$repo count=$online_count"
        else
          echo "ERROR: doctor check=runner_online reason=still_offline repo=$repo"
          failed=1
        fi
      else
        echo "ERROR: doctor check=runner_online reason=offline repo=$repo"
        failed=1
      fi
    fi
  fi

  if run_go_cmd run ./cmd/runner_health --repo-dir "$repo_dir" >/dev/null 2>&1; then
    echo "OK: doctor check=runner_health reason=ok"
  else
    if [[ "$verbose" -eq 1 ]]; then
      echo "ERROR: doctor check=runner_health reason=failed"
      run_go_cmd run ./cmd/runner_health --repo-dir "$repo_dir" || true
    else
      echo "ERROR: doctor check=runner_health reason=failed"
    fi
    if ! command -v go >/dev/null 2>&1 && ! command -v mise >/dev/null 2>&1; then
      echo "ERROR: doctor check=go reason=missing_go_and_mise"
    fi
    failed=1
  fi

  if [[ "$failed" -eq 1 ]]; then
    echo "STATUS: ERROR"
    return 1
  fi
  echo "STATUS: OK"
}

cmd_config_init() {
  local path=""
  local force=0
  local repo=""
  local project_dir="$PWD"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --path) path="${2:-}"; shift 2 ;;
      --repo) repo="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      --force) force=1; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self config-init [--path <file>] [--repo owner/repo] [--project-dir path] [--force]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for config-init: $1" >&2
        return 2
        ;;
    esac
  done

  local root="$PWD"
  root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
  [[ -z "$path" ]] && path="$root/$CONFIG_BASENAME"
  [[ -z "$repo" ]] && repo="${CONFIG_REPO:-}"
  [[ -z "$repo" ]] && repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || true)"
  [[ -z "$project_dir" || "$project_dir" == "$PWD" ]] && project_dir="$root"
  project_dir="$(expand_local_path "$project_dir")"

  if [[ -f "$path" && "$force" -ne 1 ]]; then
    echo "SKIP: config exists at $path (use --force to overwrite)"
    return 0
  fi

  mkdir -p "$(dirname "$path")"
  cat >"$path" <<EOF
# ci-self defaults (CLI options override these values)
CI_SELF_REPO=${repo}
CI_SELF_REF=main
CI_SELF_PROJECT_DIR=${project_dir}

# Optional remote defaults
CI_SELF_REMOTE_HOST=
CI_SELF_REMOTE_PROJECT_DIR=
CI_SELF_REMOTE_CLI=ci-self

# Optional runner defaults
CI_SELF_LABELS=self-hosted,mac-mini,colima,verify-full
CI_SELF_RUNNER_NAME=
CI_SELF_RUNNER_GROUP=Default
CI_SELF_DISCORD_WEBHOOK_URL=
CI_SELF_FORCE_WORKFLOW=0
CI_SELF_SKIP_WORKFLOW=0
CI_SELF_PR_BASE=main
EOF
  echo "OK: wrote config $path"
}

quote_words() {
  local out=""
  local q=""
  local arg
  for arg in "$@"; do
    printf -v q "%q" "$arg"
    if [[ -z "$out" ]]; then
      out="$q"
    else
      out="$out $q"
    fi
  done
  printf '%s\n' "$out"
}

default_remote_project_dir() {
  if [[ -n "$CONFIG_REMOTE_PROJECT_DIR" ]]; then
    printf '%s\n' "$CONFIG_REMOTE_PROJECT_DIR"
    return
  fi
  local root
  local name
  root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
  name="$(basename "$root")"
  printf '%s\n' "~/dev/$name"
}

default_local_project_dir() {
  if [[ -n "$CONFIG_PROJECT_DIR" ]]; then
    printf '%s\n' "$CONFIG_PROJECT_DIR"
    return
  fi
  git rev-parse --show-toplevel 2>/dev/null || pwd
}

sanitize_for_path_segment() {
  local raw="$1"
  local out
  out="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9._-' '-')"
  out="${out#-}"
  out="${out%-}"
  [[ -z "$out" ]] && out="remote"
  printf '%s\n' "$out"
}

remote_path_for_shell() {
  local path="$1"
  if [[ "$path" == "~/"* ]]; then
    printf '\$HOME/%s\n' "${path#~/}"
  else
    printf '%q\n' "$path"
  fi
}

run_remote_command_in_dir() {
  local host="$1"
  local project_dir="$2"
  shift 2
  local remote_cmd_q
  local script_q
  local remote_script
  local remote_cd_q

  remote_cmd_q="$(quote_words "$@")"
  remote_cd_q="$(remote_path_for_shell "$project_dir")"
  printf -v remote_script 'set -euo pipefail; cd %s; %s' "$remote_cd_q" "$remote_cmd_q"
  script_q="$(quote_words "$remote_script")"
  echo "OK: ssh host=$host dir=$project_dir cmd=$*"
  ssh "$host" "bash -lc $script_q"
}

first_existing_public_key() {
  local key=""
  for key in \
    "$HOME/.ssh/id_ed25519.pub" \
    "$HOME/.ssh/id_ecdsa.pub" \
    "$HOME/.ssh/id_rsa.pub"; do
    if [[ -f "$key" ]]; then
      printf '%s\n' "$key"
      return 0
    fi
  done
  return 1
}

ensure_ssh_key_auth() {
  local host="$1"
  if ssh -o BatchMode=yes -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no "$host" "true" >/dev/null 2>&1; then
    echo "OK: ssh key_auth host=$host"
    return 0
  fi

  echo "ERROR: ssh key-based auth failed for host=$host" >&2
  local pub_key=""
  pub_key="$(first_existing_public_key || true)"
  if [[ -n "$pub_key" ]]; then
    echo "HINT: register your public key to remote ~/.ssh/authorized_keys" >&2
    echo "HINT: cat $pub_key | ssh $host 'mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'" >&2
  else
    echo "HINT: generate a key first: ssh-keygen -t ed25519 -a 100" >&2
  fi
  return 1
}

ensure_remote_project_dir() {
  local host="$1"
  local project_dir="$2"
  local remote_dir_q
  local script_q
  local remote_script

  remote_dir_q="$(remote_path_for_shell "$project_dir")"
  printf -v remote_script 'set -euo pipefail; mkdir -p %s' "$remote_dir_q"
  script_q="$(quote_words "$remote_script")"
  echo "OK: ssh host=$host ensure_dir=$project_dir"
  ssh "$host" "bash -lc $script_q"
}

sync_local_project_to_remote() {
  local local_dir="$1"
  local host="$2"
  local project_dir="$3"
  echo "OK: rsync host=$host src=$local_dir dst=$project_dir"
  rsync -az --delete \
    --exclude ".local/" \
    --exclude "out/" \
    --exclude "cache/" \
    --exclude ".DS_Store" \
    "$local_dir/" "$host:$project_dir/"
}

fetch_remote_verify_artifacts() {
  local host="$1"
  local project_dir="$2"
  local out_dir="$3"

  mkdir -p "$out_dir" "$out_dir/logs"
  local failed=0

  if rsync -a "$host:$project_dir/out/verify-full.status" "$out_dir/"; then
    echo "OK: fetch status_file=$out_dir/verify-full.status"
  else
    echo "ERROR: fetch status_file failed host=$host path=$project_dir/out/verify-full.status" >&2
    failed=1
  fi

  if rsync -a "$host:$project_dir/out/logs/" "$out_dir/logs/"; then
    echo "OK: fetch logs_dir=$out_dir/logs"
  else
    echo "ERROR: fetch logs failed host=$host path=$project_dir/out/logs/" >&2
    failed=1
  fi

  return "$failed"
}

read_verify_status_file() {
  local status_file="$1"
  if [[ ! -f "$status_file" ]]; then
    return 0
  fi
  if grep -q "status=OK" "$status_file"; then
    echo "OK"
    return 0
  fi
  if grep -q "status=ERROR" "$status_file"; then
    echo "ERROR"
    return 0
  fi
  if grep -q "status=SKIP" "$status_file"; then
    echo "SKIP"
    return 0
  fi
}

run_remote_ci_self() {
  local host="$1"
  local project_dir="$2"
  local remote_cli="$3"
  shift 3
  local remote_args=("$@")
  local remote_args_q
  local script_q
  local remote_script
  local remote_cd_q

  remote_args_q="$(quote_words "${remote_args[@]}")"
  if [[ "$project_dir" == "~/"* ]]; then
    remote_cd_q="\$HOME/${project_dir#~/}"
  else
    printf -v remote_cd_q '%q' "$project_dir"
  fi
  printf -v remote_script 'set -euo pipefail; cd %s; %q %s' "$remote_cd_q" "$remote_cli" "$remote_args_q"
  script_q="$(quote_words "$remote_script")"
  echo "OK: ssh host=$host dir=$project_dir cmd=$remote_cli ${remote_args[*]}"
  ssh "$host" "bash -lc $script_q"
}

cmd_remote_ci() {
  local host=""
  local project_dir=""
  local local_dir=""
  local out_dir=""
  local remote_cli="ci-self"
  local repo=""
  local labels=""
  local runner_name=""
  local runner_group=""
  local discord_webhook_url=""
  local skip_bootstrap=0
  local no_sync=0
  local verify_dry_run=1
  local verify_gha_sync=1

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --host) host="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      --local-dir) local_dir="${2:-}"; shift 2 ;;
      --out-dir) out_dir="${2:-}"; shift 2 ;;
      --remote-cli) remote_cli="${2:-}"; shift 2 ;;
      --repo) repo="${2:-}"; shift 2 ;;
      --labels) labels="${2:-}"; shift 2 ;;
      --runner-name) runner_name="${2:-}"; shift 2 ;;
      --runner-group) runner_group="${2:-}"; shift 2 ;;
      --discord-webhook-url) discord_webhook_url="${2:-}"; shift 2 ;;
      --verify-dry-run) verify_dry_run="$(config_bool_to_int "${2:-}")"; shift 2 ;;
      --verify-gha-sync) verify_gha_sync="$(config_bool_to_int "${2:-}")"; shift 2 ;;
      --skip-bootstrap) skip_bootstrap=1; shift ;;
      --no-sync) no_sync=1; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self remote-ci --host <ssh-host> [--project-dir path] [--local-dir path] [--out-dir path]
                         [--repo owner/repo] [--remote-cli path]
                         [--labels csv] [--runner-name name] [--runner-group name]
                         [--discord-webhook-url url]
                         [--verify-dry-run 0|1] [--verify-gha-sync 0|1]
                         [--skip-bootstrap] [--no-sync]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for remote-ci: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$host" ]] && host="$CONFIG_REMOTE_HOST"
  [[ "$remote_cli" == "ci-self" && -n "$CONFIG_REMOTE_CLI" ]] && remote_cli="$CONFIG_REMOTE_CLI"
  [[ -z "$repo" && -n "$CONFIG_REPO" ]] && repo="$CONFIG_REPO"
  [[ -z "$labels" && -n "$CONFIG_LABELS" ]] && labels="$CONFIG_LABELS"
  [[ -z "$runner_name" && -n "$CONFIG_RUNNER_NAME" ]] && runner_name="$CONFIG_RUNNER_NAME"
  [[ -z "$runner_group" && -n "$CONFIG_RUNNER_GROUP" ]] && runner_group="$CONFIG_RUNNER_GROUP"
  [[ -z "$discord_webhook_url" && -n "$CONFIG_DISCORD_WEBHOOK_URL" ]] && discord_webhook_url="$CONFIG_DISCORD_WEBHOOK_URL"

  [[ -n "$host" ]] || { echo "ERROR: --host is required" >&2; return 2; }
  [[ -z "$project_dir" ]] && project_dir="$(default_remote_project_dir)"
  [[ -z "$local_dir" ]] && local_dir="$(default_local_project_dir)"
  local_dir="$(expand_local_path "$local_dir")"
  [[ -d "$local_dir" ]] || { echo "ERROR: --local-dir not found: $local_dir" >&2; return 2; }

  if [[ -z "$out_dir" ]]; then
    out_dir="$local_dir/out/remote/$(sanitize_for_path_segment "$host")"
  fi
  out_dir="$(expand_local_path "$out_dir")"

  command -v ssh >/dev/null 2>&1 || { echo "ERROR: ssh command not found" >&2; return 1; }
  command -v rsync >/dev/null 2>&1 || { echo "ERROR: rsync command not found" >&2; return 1; }

  ensure_ssh_key_auth "$host"
  ensure_remote_project_dir "$host" "$project_dir"

  if [[ "$no_sync" -eq 1 ]]; then
    echo "SKIP: sync reason=no_sync_flag"
  else
    sync_local_project_to_remote "$local_dir" "$host" "$project_dir"
  fi

  if [[ "$skip_bootstrap" -eq 1 ]]; then
    echo "SKIP: bootstrap reason=skip_bootstrap_flag"
  elif [[ -z "$repo" ]]; then
    echo "SKIP: bootstrap reason=repo_not_set"
  else
    local register_args=(register --repo "$repo" --repo-dir "$project_dir" --skip-workflow --skip-dispatch)
    [[ -n "$labels" ]] && register_args+=(--labels "$labels")
    [[ -n "$runner_name" ]] && register_args+=(--runner-name "$runner_name")
    [[ -n "$runner_group" ]] && register_args+=(--runner-group "$runner_group")
    [[ -n "$discord_webhook_url" ]] && register_args+=(--discord-webhook-url "$discord_webhook_url")
    if ! run_remote_ci_self "$host" "$project_dir" "$remote_cli" "${register_args[@]}"; then
      echo "WARN: bootstrap failed; continuing standalone verify" >&2
    fi
  fi

  local sha=""
  local ref=""
  sha="$(git -C "$local_dir" rev-parse HEAD 2>/dev/null || true)"
  ref="$(git -C "$local_dir" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  [[ "$ref" == "HEAD" ]] && ref=""

  local remote_verify_args=(env "VERIFY_DRY_RUN=$verify_dry_run" "VERIFY_GHA_SYNC=$verify_gha_sync" "GITHUB_ACTIONS=true")
  [[ -n "$sha" ]] && remote_verify_args+=("GITHUB_SHA=$sha")
  [[ -n "$ref" ]] && remote_verify_args+=("GITHUB_REF_NAME=$ref")
  remote_verify_args+=(sh ops/ci/run_verify_full.sh)

  local verify_failed=0
  if ! run_remote_command_in_dir "$host" "$project_dir" "${remote_verify_args[@]}"; then
    echo "ERROR: remote verify command failed" >&2
    verify_failed=1
  fi

  local fetch_failed=0
  if ! fetch_remote_verify_artifacts "$host" "$project_dir" "$out_dir"; then
    fetch_failed=1
  fi

  local status_file="$out_dir/verify-full.status"
  local verify_status=""
  verify_status="$(read_verify_status_file "$status_file")"
  if [[ -z "$verify_status" ]]; then
    echo "ERROR: verify status missing in $status_file" >&2
    return 1
  fi

  echo "OK: remote-ci result status=$verify_status status_file=$status_file"
  if [[ "$verify_failed" -eq 1 || "$fetch_failed" -eq 1 || "$verify_status" != "OK" ]]; then
    return 1
  fi
  return 0
}

cmd_remote_register() {
  local host=""
  local project_dir=""
  local remote_cli="ci-self"
  local repo=""
  local labels=""
  local runner_name=""
  local runner_group=""
  local discord_webhook_url=""
  local force_workflow=0
  local skip_workflow=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --host) host="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      --remote-cli) remote_cli="${2:-}"; shift 2 ;;
      --repo) repo="${2:-}"; shift 2 ;;
      --labels) labels="${2:-}"; shift 2 ;;
      --runner-name) runner_name="${2:-}"; shift 2 ;;
      --runner-group) runner_group="${2:-}"; shift 2 ;;
      --discord-webhook-url) discord_webhook_url="${2:-}"; shift 2 ;;
      --force-workflow) force_workflow=1; shift ;;
      --skip-workflow) skip_workflow=1; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self remote-register --host <ssh-host> [--project-dir path] [--repo owner/repo] [--remote-cli path]
                               [--labels csv] [--runner-name name] [--runner-group name]
                               [--discord-webhook-url url] [--force-workflow] [--skip-workflow]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for remote-register: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$host" ]] && host="$CONFIG_REMOTE_HOST"
  [[ "$remote_cli" == "ci-self" && -n "$CONFIG_REMOTE_CLI" ]] && remote_cli="$CONFIG_REMOTE_CLI"
  [[ -z "$repo" && -n "$CONFIG_REPO" ]] && repo="$CONFIG_REPO"
  [[ -z "$labels" && -n "$CONFIG_LABELS" ]] && labels="$CONFIG_LABELS"
  [[ -z "$runner_name" && -n "$CONFIG_RUNNER_NAME" ]] && runner_name="$CONFIG_RUNNER_NAME"
  [[ -z "$runner_group" && -n "$CONFIG_RUNNER_GROUP" ]] && runner_group="$CONFIG_RUNNER_GROUP"
  [[ -z "$discord_webhook_url" && -n "$CONFIG_DISCORD_WEBHOOK_URL" ]] && discord_webhook_url="$CONFIG_DISCORD_WEBHOOK_URL"
  [[ "$force_workflow" -eq 0 ]] && force_workflow="$(config_bool_to_int "$CONFIG_FORCE_WORKFLOW")"
  [[ "$skip_workflow" -eq 0 ]] && skip_workflow="$(config_bool_to_int "$CONFIG_SKIP_WORKFLOW")"

  [[ -n "$host" ]] || { echo "ERROR: --host is required" >&2; return 2; }
  if [[ -z "$project_dir" ]]; then
    project_dir="$(default_remote_project_dir)"
  fi

  local args=(register)
  [[ -n "$repo" ]] && args+=(--repo "$repo")
  [[ -n "$labels" ]] && args+=(--labels "$labels")
  [[ -n "$runner_name" ]] && args+=(--runner-name "$runner_name")
  [[ -n "$runner_group" ]] && args+=(--runner-group "$runner_group")
  [[ -n "$discord_webhook_url" ]] && args+=(--discord-webhook-url "$discord_webhook_url")
  [[ "$force_workflow" -eq 1 ]] && args+=(--force-workflow)
  [[ "$skip_workflow" -eq 1 ]] && args+=(--skip-workflow)

  run_remote_ci_self "$host" "$project_dir" "$remote_cli" "${args[@]}"
}

cmd_remote_run_focus() {
  local host=""
  local project_dir=""
  local remote_cli="ci-self"
  local repo=""
  local ref=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --host) host="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      --remote-cli) remote_cli="${2:-}"; shift 2 ;;
      --repo) repo="${2:-}"; shift 2 ;;
      --ref) ref="${2:-}"; shift 2 ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self remote-run-focus --host <ssh-host> [--project-dir path] [--repo owner/repo] [--ref branch] [--remote-cli path]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for remote-run-focus: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$host" ]] && host="$CONFIG_REMOTE_HOST"
  [[ "$remote_cli" == "ci-self" && -n "$CONFIG_REMOTE_CLI" ]] && remote_cli="$CONFIG_REMOTE_CLI"
  [[ -z "$repo" && -n "$CONFIG_REPO" ]] && repo="$CONFIG_REPO"
  [[ -z "$ref" ]] && ref="$(resolve_ref "$ref")"

  [[ -n "$host" ]] || { echo "ERROR: --host is required" >&2; return 2; }
  if [[ -z "$project_dir" ]]; then
    project_dir="$(default_remote_project_dir)"
  fi

  local args=(run-focus --ref "$ref")
  [[ -n "$repo" ]] && args+=(--repo "$repo")
  run_remote_ci_self "$host" "$project_dir" "$remote_cli" "${args[@]}"
}

cmd_remote_up() {
  local host=""
  local project_dir=""
  local remote_cli="ci-self"
  local repo=""
  local ref=""
  local labels=""
  local runner_name=""
  local runner_group=""
  local discord_webhook_url=""
  local force_workflow=0
  local skip_workflow=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --host) host="${2:-}"; shift 2 ;;
      --project-dir) project_dir="${2:-}"; shift 2 ;;
      --remote-cli) remote_cli="${2:-}"; shift 2 ;;
      --repo) repo="${2:-}"; shift 2 ;;
      --ref) ref="${2:-}"; shift 2 ;;
      --labels) labels="${2:-}"; shift 2 ;;
      --runner-name) runner_name="${2:-}"; shift 2 ;;
      --runner-group) runner_group="${2:-}"; shift 2 ;;
      --discord-webhook-url) discord_webhook_url="${2:-}"; shift 2 ;;
      --force-workflow) force_workflow=1; shift ;;
      --skip-workflow) skip_workflow=1; shift ;;
      -h|--help)
        cat <<'USAGE'
Usage: ci-self remote-up --host <ssh-host> [--project-dir path] [--repo owner/repo] [--ref branch]
                         [--remote-cli path] [--labels csv] [--runner-name name]
                         [--runner-group name] [--discord-webhook-url url]
                         [--force-workflow] [--skip-workflow]
USAGE
        return 0
        ;;
      *)
        echo "ERROR: unknown option for remote-up: $1" >&2
        return 2
        ;;
    esac
  done

  [[ -z "$host" ]] && host="$CONFIG_REMOTE_HOST"
  [[ "$remote_cli" == "ci-self" && -n "$CONFIG_REMOTE_CLI" ]] && remote_cli="$CONFIG_REMOTE_CLI"
  [[ -z "$repo" && -n "$CONFIG_REPO" ]] && repo="$CONFIG_REPO"
  [[ -z "$ref" ]] && ref="$(resolve_ref "$ref")"
  [[ -z "$labels" && -n "$CONFIG_LABELS" ]] && labels="$CONFIG_LABELS"
  [[ -z "$runner_name" && -n "$CONFIG_RUNNER_NAME" ]] && runner_name="$CONFIG_RUNNER_NAME"
  [[ -z "$runner_group" && -n "$CONFIG_RUNNER_GROUP" ]] && runner_group="$CONFIG_RUNNER_GROUP"
  [[ -z "$discord_webhook_url" && -n "$CONFIG_DISCORD_WEBHOOK_URL" ]] && discord_webhook_url="$CONFIG_DISCORD_WEBHOOK_URL"
  [[ "$force_workflow" -eq 0 ]] && force_workflow="$(config_bool_to_int "$CONFIG_FORCE_WORKFLOW")"
  [[ "$skip_workflow" -eq 0 ]] && skip_workflow="$(config_bool_to_int "$CONFIG_SKIP_WORKFLOW")"

  [[ -n "$host" ]] || { echo "ERROR: --host is required" >&2; return 2; }
  if [[ -z "$project_dir" ]]; then
    project_dir="$(default_remote_project_dir)"
  fi

  local register_args=(--host "$host" --project-dir "$project_dir" --remote-cli "$remote_cli")
  [[ -n "$repo" ]] && register_args+=(--repo "$repo")
  [[ -n "$labels" ]] && register_args+=(--labels "$labels")
  [[ -n "$runner_name" ]] && register_args+=(--runner-name "$runner_name")
  [[ -n "$runner_group" ]] && register_args+=(--runner-group "$runner_group")
  [[ -n "$discord_webhook_url" ]] && register_args+=(--discord-webhook-url "$discord_webhook_url")
  [[ "$force_workflow" -eq 1 ]] && register_args+=(--force-workflow)
  [[ "$skip_workflow" -eq 1 ]] && register_args+=(--skip-workflow)
  cmd_remote_register "${register_args[@]}"

  local run_focus_args=(--host "$host" --project-dir "$project_dir" --remote-cli "$remote_cli" --ref "$ref")
  [[ -n "$repo" ]] && run_focus_args+=(--repo "$repo")
  cmd_remote_run_focus "${run_focus_args[@]}"
}

main() {
  local cmd="${1:-help}"
  shift || true
  case "$cmd" in
    up) cmd_up "$@" ;;
    focus) cmd_focus "$@" ;;
    doctor) cmd_doctor "$@" ;;
    config-init) cmd_config_init "$@" ;;
    register) cmd_register "$@" ;;
    run-watch) cmd_run_watch "$@" ;;
    run-focus) cmd_run_watch --all-green --sync-pr-template "$@" ;;
    remote-ci) cmd_remote_ci "$@" ;;
    remote-register) cmd_remote_register "$@" ;;
    remote-run-focus) cmd_remote_run_focus "$@" ;;
    remote-up) cmd_remote_up "$@" ;;
    watch) cmd_watch "$@" ;;
    help|-h|--help) usage ;;
    *)
      echo "ERROR: unknown command: $cmd" >&2
      usage >&2
      return 2
      ;;
  esac
}

load_config
main "$@"
