#!/usr/bin/env sh

REPO_DIR="${REPO_DIR:-$PWD}"

cd "${REPO_DIR}"
go run ./cmd/verify-lite
