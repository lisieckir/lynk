# Lynk — multi-stage Dockerfile (docs/SPEC.md section 8, architecture notes).
#
# Two stages: a pinned Go builder that compiles the app, and a minimal pinned
# Alpine runtime with busybox, CA certificates, a non-root user and /data as
# the persistence target. The final image contains no developer toolchain.
#
# Reproducibility:
#   - Images are digest-pinned (BUILDER_IMAGE, RUNTIME_IMAGE below).
#   - `go build -trimpath` yields a reproducible binary; modernc.org/sqlite
#     is pure Go, so the default build is static (CGO_ENABLED=0).
#   - busybox wget (used by HEALTHCHECK and available in the shell) is part
#     of the digest-pinned alpine base.

ARG BUILDER_IMAGE=golang:1.27.4-bookworm@sha256:ce59b41415e2524ae99926fe1f37ce48ba8c194dcbd2a56a65436cdcefa2bd34
ARG RUNTIME_IMAGE=alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc

# --- Stage 1: build --------------------------------------------------------
FROM ${BUILDER_IMAGE} AS build

WORKDIR /src

# Resolve dependencies first so module downloads cache as their own layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static where practical: the app and modernc.org/sqlite use only net/http,
# database/sql and the Go standard library, so the default build needs no
# cgo. trimpath also removes filesystem paths from the binary.
ENV CGO_ENABLED=0
ARG VERSION=dev
RUN go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /usr/local/bin/lynk ./cmd/lynk

# --- Stage 2: runtime ------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS runtime

# CA certificates: alpine 3.20.10 (the digest-pinned tag above) ships
# /etc/ssl/certs/ca-certificates.crt, which is on Go's default Linux
# certificate paths, so HTTPS metadata fetching has trust anchors.

# Non-root runtime user (uid 10152). busybox adduser/addgroup semantics were
# validated against the alpine base image (loginutils/adduser.c,
# loginutils/addgroup.c): addgroup creates the group, adduser -S creates a
# system user in it, -H skips creating the home directory. The app creates
# its own data directory contents; only /data itself is chowned here.
RUN addgroup -S -g 10152 lynk \
    && adduser -S -u 10152 -G lynk -H lynk \
    && mkdir -p /data && chown lynk:lynk /data

COPY --from=build /usr/local/bin/lynk /usr/local/bin/lynk

# Web assets (web/templates, web/static) are embedded or copied into the
# binary via //go:embed in internal/web once that package exists; the build
# stage already includes them through `COPY . .`, so no additional copy step
# is needed today.

EXPOSE 9090

# Healthcheck against the app's readiness endpoint (/health,
# internal/health). Container port 9090 is the app's fixed default; the
# published host port is configurable in compose.yaml via LYNK_PORT.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:9090/health"]

USER 10152

ENTRYPOINT ["/usr/local/bin/lynk"]
