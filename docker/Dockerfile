# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Install CA certs for outbound HTTPS (Claude API) and git for go get
RUN apk add --no-cache ca-certificates git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both binaries — CGO disabled, stripped for smallest possible image
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/worker  ./cmd/worker

# ── Stage 2: gateway image ────────────────────────────────────────────────────
FROM scratch AS gateway
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/gateway /gateway
EXPOSE 8080
ENTRYPOINT ["/gateway"]

# ── Stage 3: worker image ─────────────────────────────────────────────────────
FROM scratch AS worker
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/worker /worker
ENTRYPOINT ["/worker"]
