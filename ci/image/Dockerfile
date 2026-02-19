# syntax=docker/dockerfile:1

# ---- go-builder ----
FROM golang:1.25.6-bookworm@sha256:f4490d7b261d73af4543c46ac6597d7d101b6e1755bcdd8c5159fda7046b6b3e AS go-builder
WORKDIR /src
COPY go.mod ./
COPY cmd/verify-full/main.go ./cmd/verify-full/main.go
# If this image switches to mise-based toolchain install,
# run `mise trust` before `mise install`.
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/verify-full ./cmd/verify-full

# ---- base ----
FROM debian:stable-slim@sha256:4448d44b91bf4a13cb1b4e02d9d5f87ed40621d6e33f0ae7b6ddf71d57e29364 AS base
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates coreutils git curl tzdata \
  && rm -rf /var/lib/apt/lists/*

# ---- runtime ----
FROM base AS runtime
WORKDIR /repo
# Keep lock metadata inside image for traceability.
COPY ci/image/versions.lock /etc/ci/versions.lock
COPY --from=go-builder /out/verify-full /usr/local/bin/verify-full

# non-root user (minimal)
RUN chmod +x /usr/local/bin/verify-full \
  && useradd -m -u 10001 ci \
  && mkdir -p /out /cache /repo \
  && chown -R ci:ci /out /cache /repo
USER ci

# Contract:
# - /repo : bind mount
# - /out  : bind mount
# - /cache: volume
CMD ["/usr/local/bin/verify-full"]
