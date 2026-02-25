#!/usr/bin/env sh
# Shell極薄: ホストラッパ経由で verify-lite を実行する
# SOT: out/verify-lite.status

REPO_DIR="${REPO_DIR:-$PWD}"

cd "${REPO_DIR}"
go run ./cmd/verify_lite_host
