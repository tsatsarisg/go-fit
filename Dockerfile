# syntax=docker/dockerfile:1.7
#
# Multi-stage build for go-fit.
#
# Builder: alpine picked over bookworm — smaller pulls, faster CI layer cache,
# and we compile statically (CGO_ENABLED=0) so glibc-vs-musl is irrelevant.
# Final:   distroless static nonroot — no shell, no package manager, no libc.
#          Minimal attack surface. Healthcheck is intentionally absent from
#          the image (see HEALTHCHECK note below) — wire it at the
#          compose/orchestrator layer instead.

ARG GO_VERSION=1.24.4

# ---------------------------------------------------------------------------
# Stage: deps — cached go mod download layer
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS deps
WORKDIR /src

# ca-certificates and git are only needed if we have VCS-pinned deps or need
# TLS during `go mod download`. Cheap to include.
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# ---------------------------------------------------------------------------
# Stage: builder — compile the static binary
# ---------------------------------------------------------------------------
FROM deps AS builder
WORKDIR /src

ARG TARGETOS
ARG TARGETARCH
ARG GIT_SHA=unknown
ARG BUILD_DATE=unknown
ARG VERSION=dev

COPY . .

# -trimpath strips filesystem paths from the binary.
# -ldflags '-s -w -buildid=' strips the symbol table, DWARF, and the build id
#   (reproducibility + smaller image).
# CGO_ENABLED=0 produces a static binary runnable on distroless/static.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build \
        -trimpath \
        -ldflags="-s -w -buildid= -X main.version=${VERSION} -X main.commit=${GIT_SHA} -X main.buildDate=${BUILD_DATE}" \
        -o /out/api \
        ./cmd/api

# ---------------------------------------------------------------------------
# Stage: dev — hot-reload via air, used by docker-compose.yml for local dev
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS dev
WORKDIR /app

RUN apk add --no-cache git curl \
 && go install github.com/air-verse/air@latest

# Source gets bind-mounted by compose; we just warm the module cache here so
# the first `air` run doesn't redownload everything.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

EXPOSE 8080
ENV PORT=8080 APP_ENV=development
ENTRYPOINT ["air", "-c", ".air.toml"]

# ---------------------------------------------------------------------------
# Stage: final — minimal, non-root, production runtime
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS final

ARG GIT_SHA=unknown
ARG BUILD_DATE=unknown
ARG VERSION=dev

# OCI image labels — consumed by registries, sboms, and humans.
LABEL org.opencontainers.image.title="go-fit" \
      org.opencontainers.image.description="go-fit HTTP API server" \
      org.opencontainers.image.source="https://github.com/tsatsarisg/go-fit" \
      org.opencontainers.image.url="https://github.com/tsatsarisg/go-fit" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.revision="${GIT_SHA}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.vendor="tsatsarisg"

WORKDIR /app
COPY --from=builder /out/api /app/api

EXPOSE 8080
ENV PORT=8080 APP_ENV=production

# Distroless has no shell, so a Dockerfile HEALTHCHECK that invokes a shell
# command cannot work here. Compose/Kubernetes should probe /health directly
# over HTTP (see docker-compose.prod.yml and any k8s Probe spec).

USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
