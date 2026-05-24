#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scaffold_mobile_build_workflow.sh [--repo <path>] [--apply] [--force]

Options:
  --repo <path>  Target repository path (default: current directory)
  --apply        Write .github/workflows/mobile-build.yml to target repository
  --force        Overwrite mobile-build.yml when it already exists (requires --apply)
  -h, --help     Show this help

Behavior:
  - Generates a fastlane-based iOS/Android workflow for main and develop.
  - Without --apply, prints generated YAML to stdout.
USAGE
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
WORKFLOW_DIR="$REPO_DIR/.github/workflows"
WORKFLOW_FILE="$WORKFLOW_DIR/mobile-build.yml"

render_mobile_build() {
  cat <<'YAML'
name: mobile-build

on:
  workflow_dispatch:
    inputs:
      platform:
        type: choice
        required: true
        default: all
        options:
          - all
          - ios
          - android
      lane:
        type: string
        required: true
        default: ci
  push:
    branches: ["main", "develop"]
    paths:
      - ".github/workflows/mobile-build.yml"
      - "Gemfile"
      - "Gemfile.lock"
      - "fastlane/**"
      - "ios/**"
      - "android/**"
  pull_request:
    branches: ["main", "develop"]
    paths:
      - ".github/workflows/mobile-build.yml"
      - "Gemfile"
      - "Gemfile.lock"
      - "fastlane/**"
      - "ios/**"
      - "android/**"

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}-${{ github.event.inputs.platform || 'auto' }}
  cancel-in-progress: true

env:
  BUNDLE_PATH: vendor/bundle
  FASTLANE_HIDE_CHANGELOG: "1"
  FASTLANE_SKIP_UPDATE_CHECK: "1"
  FASTLANE_LANE: ${{ github.event_name == 'workflow_dispatch' && inputs.lane || 'ci' }}

jobs:
  ios:
    name: iOS fastlane
    if: ${{ github.event.act == true || (vars.SELF_HOSTED_OWNER != '' && github.repository_owner == vars.SELF_HOSTED_OWNER && (github.event_name != 'pull_request' || github.event.pull_request.head.repo.fork == false) && (github.event_name != 'workflow_dispatch' || inputs.platform == 'all' || inputs.platform == 'ios')) }}
    runs-on:
      - self-hosted
      - mac-mini
      - mobile
      - ios
      - fastlane
      - xcode
    timeout-minutes: 90
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5

      - name: Bundle install
        shell: bash
        run: |
          test -f Gemfile || { echo "ERROR: Gemfile is required for mobile fastlane workflow"; exit 1; }
          bundle check || bundle install --jobs 4 --retry 3

      - name: Fastlane iOS
        shell: bash
        run: |
          bundle exec fastlane ios "$FASTLANE_LANE"

  android:
    name: Android fastlane
    if: ${{ github.event.act == true || (vars.SELF_HOSTED_OWNER != '' && github.repository_owner == vars.SELF_HOSTED_OWNER && (github.event_name != 'pull_request' || github.event.pull_request.head.repo.fork == false) && (github.event_name != 'workflow_dispatch' || inputs.platform == 'all' || inputs.platform == 'android')) }}
    runs-on:
      - self-hosted
      - mac-mini
      - mobile
      - android
      - fastlane
      - android-sdk
    timeout-minutes: 90
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5

      - name: Bundle install
        shell: bash
        run: |
          test -f Gemfile || { echo "ERROR: Gemfile is required for mobile fastlane workflow"; exit 1; }
          bundle check || bundle install --jobs 4 --retry 3

      - name: Fastlane Android
        shell: bash
        run: |
          bundle exec fastlane android "$FASTLANE_LANE"
YAML
}

if [[ "$APPLY" -eq 0 ]]; then
  render_mobile_build
  exit 0
fi

if [[ -f "$WORKFLOW_FILE" && "$FORCE" -ne 1 ]]; then
  echo "SKIP: $WORKFLOW_FILE already exists (use --force to overwrite)"
  exit 0
fi

mkdir -p "$WORKFLOW_DIR"
render_mobile_build >"$WORKFLOW_FILE"
echo "OK: wrote $WORKFLOW_FILE"
