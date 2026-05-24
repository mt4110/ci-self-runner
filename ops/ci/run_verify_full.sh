#!/usr/bin/env sh
# Docker経由で verify-full を実行し、ホスト側のstatus契約を保つ
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
STATUS_PATH="${OUT_DIR}/verify-full.status"
DOCKER_READY_REASON="docker_daemon_unavailable"

mkdir -p "${OUT_DIR}"
rm -f "${STATUS_PATH}"

verify_mode() {
  if [ "${VERIFY_DRY_RUN}" = "1" ]; then
    echo "dry-run"
  else
    echo "full"
  fi
}

gha_sync_value() {
  if [ "${VERIFY_GHA_SYNC}" = "1" ] || [ "${GITHUB_ACTIONS}" = "true" ]; then
    echo "true"
  else
    echo "false"
  fi
}

write_error_status() {
  reason="$1"
  stamp="$(date -u '+%Y%m%dT%H%M%SZ')"
  mode="$(verify_mode)"
  gha_sync="$(gha_sync_value)"
  {
    echo "ERROR: verify-full status=ERROR mode=${mode}"
    echo "timestamp=${stamp}"
    echo "status=ERROR"
    echo "mode=${mode}"
    echo "gha_sync=${gha_sync}"
    if [ -n "${GITHUB_RUN_ID}" ]; then
      echo "OK: github_run_id=${GITHUB_RUN_ID}"
      echo "github_run_id=${GITHUB_RUN_ID}"
    fi
    if [ -n "${GITHUB_SHA}" ]; then
      echo "OK: github_sha=${GITHUB_SHA}"
      echo "github_sha=${GITHUB_SHA}"
    fi
    if [ -n "${GITHUB_REF_NAME}" ]; then
      echo "OK: github_ref=${GITHUB_REF_NAME}"
      echo "github_ref=${GITHUB_REF_NAME}"
    fi
    echo "source=run_verify_full"
    echo "ERROR: reason=${reason}"
    echo "reason=${reason}"
  } >"${STATUS_PATH}"
}

ensure_docker_ready() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker command not found" >&2
    DOCKER_READY_REASON="docker_command_missing"
    return 1
  fi

  if docker info >/dev/null 2>&1; then
    return 0
  fi

  if command -v colima >/dev/null 2>&1; then
    echo "WARN: docker daemon unavailable; attempting colima start" >&2
    colima status >/dev/null 2>&1 || colima start
    if docker info >/dev/null 2>&1; then
      echo "OK: docker daemon available after colima start"
      return 0
    fi
  fi

  echo "ERROR: docker daemon unavailable" >&2
  DOCKER_READY_REASON="docker_daemon_unavailable"
  return 1
}

if ! ensure_docker_ready; then
  write_error_status "${DOCKER_READY_REASON}"
  exit 1
fi

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
rc="$?"

if [ "${rc}" -ne 0 ] && [ ! -f "${STATUS_PATH}" ]; then
  write_error_status "docker_run_failed"
fi

exit "${rc}"
