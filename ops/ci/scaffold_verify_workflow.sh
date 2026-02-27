#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scaffold_verify_workflow.sh [--repo <path>] [--apply] [--force] [--skip-gitignore]

Options:
  --repo <path>  Target repository path (default: current directory)
  --apply        Write .github/workflows/verify.yml to target repository
  --force        Overwrite verify.yml when it already exists (requires --apply)
  --skip-gitignore  Do not update .gitignore with local runtime dirs
  -h, --help     Show this help

Behavior:
  - Auto-detects workflow mode:
    - nix mode: flake.nix exists
    - go mode: go.mod exists
    - minimal mode: fallback
  - Without --apply, prints generated YAML to stdout.
USAGE
}

REPO_DIR="."
APPLY=0
FORCE=0
UPDATE_GITIGNORE=1

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
    --skip-gitignore)
      UPDATE_GITIGNORE=0
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
WORKFLOW_DIR="$REPO_DIR/.github/workflows"
WORKFLOW_FILE="$WORKFLOW_DIR/verify.yml"
GITIGNORE_FILE="$REPO_DIR/.gitignore"

MODE="minimal"
if [[ -f "$REPO_DIR/flake.nix" ]]; then
  MODE="nix"
elif [[ -f "$REPO_DIR/go.mod" ]]; then
  MODE="go"
fi

render_go() {
  cat <<'YAML'
name: verify

on:
  workflow_dispatch:
  push:
    branches: ["main"]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true

jobs:
  verify:
    if: ${{ vars.SELF_HOSTED_OWNER != '' && github.repository_owner == vars.SELF_HOSTED_OWNER && (github.event_name != 'pull_request' || github.event.pull_request.head.repo.fork == false) }}
    runs-on:
      - self-hosted
      - mac-mini
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Verify
        run: |
          go test ./...
          go vet ./...
YAML
}

render_nix() {
  cat <<'YAML'
name: verify

on:
  workflow_dispatch:
  push:
    branches: ["main"]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true

jobs:
  verify:
    if: ${{ vars.SELF_HOSTED_OWNER != '' && github.repository_owner == vars.SELF_HOSTED_OWNER && (github.event_name != 'pull_request' || github.event.pull_request.head.repo.fork == false) }}
    runs-on:
      - self-hosted
      - mac-mini
    timeout-minutes: 45
    steps:
      - uses: actions/checkout@v4

      - name: Verify (Nix)
        shell: bash
        run: |
          if ! command -v nix >/dev/null 2>&1 && [[ -f /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh ]]; then
            # shellcheck disable=SC1091
            . /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
          fi
          if ! command -v nix >/dev/null 2>&1; then
            export PATH="/nix/var/nix/profiles/default/bin:/nix/var/nix/profiles/per-user/${USER:-$(id -un)}/profile/bin:$PATH"
          fi
          if ! command -v nix >/dev/null 2>&1; then
            echo "ERROR: nix is required on this runner"
            echo "HINT: install nix and/or ensure nix-daemon profile exists"
            exit 1
          fi
          nix shell nixpkgs#go nixpkgs#uv nixpkgs#python312 -c env -u GOROOT -u GOTOOLDIR -u GOENV bash -c '
            uv sync
            make test
            make smoke
            go vet ./...
          '
YAML
}

render_minimal() {
  cat <<'YAML'
name: verify

on:
  workflow_dispatch:
  push:
    branches: ["main"]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true

jobs:
  verify:
    if: ${{ vars.SELF_HOSTED_OWNER != '' && github.repository_owner == vars.SELF_HOSTED_OWNER && (github.event_name != 'pull_request' || github.event.pull_request.head.repo.fork == false) }}
    runs-on:
      - self-hosted
      - mac-mini
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v4

      - name: Verify
        run: |
          echo "TODO: replace this step with your project verify command"
          exit 1
YAML
}

TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

case "$MODE" in
  go) render_go >"$TMP_FILE" ;;
  nix) render_nix >"$TMP_FILE" ;;
  *) render_minimal >"$TMP_FILE" ;;
esac

if [[ "$APPLY" -eq 0 ]]; then
  cat "$TMP_FILE"
  exit 0
fi

mkdir -p "$WORKFLOW_DIR"
if [[ -f "$WORKFLOW_FILE" && "$FORCE" -ne 1 ]]; then
  echo "SKIP: $WORKFLOW_FILE already exists (use --force to overwrite)"
  if [[ "$MODE" == "nix" ]]; then
    if ! grep -Fq "nix-daemon.sh" "$WORKFLOW_FILE"; then
      echo "WARN: existing verify.yml may not load nix in non-login shells"
      echo "HINT: rerun with --force to refresh verify.yml template"
    fi
  fi
  if [[ "$UPDATE_GITIGNORE" -eq 1 ]]; then
    touch "$GITIGNORE_FILE"
    for entry in ".local/" "out/" "cache/"; do
      if ! grep -Fxq "$entry" "$GITIGNORE_FILE"; then
        printf '%s\n' "$entry" >>"$GITIGNORE_FILE"
        echo "OK: added to .gitignore: $entry"
      fi
    done
  fi
  exit 0
fi

cp "$TMP_FILE" "$WORKFLOW_FILE"
echo "OK: wrote $WORKFLOW_FILE (mode=$MODE)"

if [[ "$UPDATE_GITIGNORE" -eq 1 ]]; then
  touch "$GITIGNORE_FILE"
  for entry in ".local/" "out/" "cache/"; do
    if ! grep -Fxq "$entry" "$GITIGNORE_FILE"; then
      printf '%s\n' "$entry" >>"$GITIGNORE_FILE"
      echo "OK: added to .gitignore: $entry"
    fi
  done
fi
