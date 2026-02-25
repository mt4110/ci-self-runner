#!/usr/bin/env sh
# Shell極薄: Docker経由で verify-full を実行する
# SOT: out/verify-full.status
# ホスト側ラッパ: go run ./cmd/verify_full_host (--dry-run)

IMAGE="${IMAGE:-ci-self-runner:local}"
REPO_DIR="${REPO_DIR:-$PWD}"
OUT_DIR="${OUT_DIR:-$PWD/out}"
CACHE_VOL="${CACHE_VOL:-ci-cache}"
VERIFY_DRY_RUN="${VERIFY_DRY_RUN:-0}"
VERIFY_GHA_SYNC="${VERIFY_GHA_SYNC:-0}"
GITHUB_ACTIONS="${GITHUB_ACTIONS:-false}"
GITHUB_RUN_ID="${GITHUB_RUN_ID:-}"
GITHUB_SHA="${GITHUB_SHA:-}"
GITHUB_REF_NAME="${GITHUB_REF_NAME:-}"
HOST_UID="${HOST_UID:-$(id -u)}"
HOST_GID="${HOST_GID:-$(id -g)}"

mkdir -p "${OUT_DIR}"

docker run --rm \
  --user "${HOST_UID}:${HOST_GID}" \
  -e VERIFY_DRY_RUN="${VERIFY_DRY_RUN}" \
  -e VERIFY_GHA_SYNC="${VERIFY_GHA_SYNC}" \
  -e GITHUB_ACTIONS="${GITHUB_ACTIONS}" \
  -e GITHUB_RUN_ID="${GITHUB_RUN_ID}" \
  -e GITHUB_SHA="${GITHUB_SHA}" \
  -e GITHUB_REF_NAME="${GITHUB_REF_NAME}" \
  -v "${REPO_DIR}:/repo" \
  -v "${OUT_DIR}:/out" \
  -v "${CACHE_VOL}:/cache" \
  -w /repo \
  "${IMAGE}" \
  /usr/local/bin/verify-full
