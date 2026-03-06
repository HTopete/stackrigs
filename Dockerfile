# StackRigs — Multi-stage production build
# Supports linux/arm64 (Pi 5) and linux/amd64 (VPS) via Docker buildx.
#
# Build for current platform:
#   docker build -t stackrigs .
#
# Build multi-arch and push:
#   docker buildx build --platform linux/arm64,linux/amd64 -t ghcr.io/your-org/stackrigs:latest --push .

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata sqlite

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO disabled: uses a pure-Go SQLite driver (modernc.org/sqlite or similar).
# TARGETARCH is injected by Docker buildx automatically.
ARG TARGETOS=linux
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -trimpath -o /stackrigs ./cmd/server

# --- Runtime ---
FROM scratch

# TLS certificates for outbound HTTPS (GitHub OAuth, R2 uploads, etc.)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Timezone data so time.LoadLocation works in scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /stackrigs /stackrigs

ENV TZ=UTC
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/stackrigs", "-healthcheck"]

ENTRYPOINT ["/stackrigs"]
