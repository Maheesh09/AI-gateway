# AI-Powered API Gateway

A production-grade reverse proxy written in Go. Handles JWT authentication, per-key rate limiting using a sliding window algorithm, dynamic route management, and an async AI layer that detects and explains suspicious traffic patterns using Claude.

Built as a personal backend engineering project to demonstrate real distributed systems concepts — not a CRUD app.

---

## What it does

Incoming requests hit the gateway, pass through an ordered middleware pipeline (auth → rate limit → route match → proxy), and are forwarded to registered upstream services. After each request is proxied, a background job is dispatched to an AI worker that analyses recent traffic patterns. If the traffic looks suspicious — burst activity, high error rates, scanning behaviour — the worker calls the Claude API, which generates a plain-English explanation and a severity rating. CRITICAL alerts automatically disable the offending API key until an admin resolves it.

```
Client → Auth middleware → Rate limiter → Route matcher → Reverse proxy → Upstream
                                                                    ↓
                                                        Async job (asynq/Redis)
                                                                    ↓
                                                        Rule-based detector
                                                                    ↓
                                                        Claude API (if triggered)
                                                                    ↓
                                                        Anomaly alert stored in DB
```

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| HTTP router | chi |
| Database | PostgreSQL 16 (pgx/v5 driver, pgxpool) |
| Cache / queue store | Redis 7 |
| Background jobs | asynq |
| Authentication | JWT (golang-jwt/jwt) + SHA-256 hashed API keys |
| AI analysis | Anthropic Claude API |
| Metrics | Prometheus (client_golang) |
| Logging | zap (structured JSON) |
| Migrations | golang-migrate |
| Containerisation | Docker + Docker Compose |

---

## Architecture

Two independent Go binaries, both compiled from the same repository:

**`cmd/gateway`** — handles live HTTP traffic. Synchronous, latency-sensitive. Authenticates requests, enforces rate limits, proxies to upstream services, and enqueues analysis jobs.

**`cmd/worker`** — processes background jobs from the Redis queue. Runs the anomaly detector and calls Claude when rules are triggered. Decoupled from the gateway so it can be scaled and restarted independently without affecting traffic.

```
ai-gateway/
├── cmd/
│   ├── gateway/main.go       ← gateway binary
│   └── worker/main.go        ← AI worker binary
├── internal/
│   ├── config/               ← env parsing, fail-fast validation
│   ├── db/                   ← pgxpool + Redis client + Lua scripts
│   ├── middleware/            ← auth, rate limiter, logger, recovery
│   ├── proxy/                ← route matching, httputil.ReverseProxy wrapper
│   ├── ai/                   ← anomaly detector, Claude analyzer, asynq worker
│   ├── handler/              ← admin API handlers
│   ├── repository/           ← all SQL queries
│   └── service/              ← business logic layer
├── migrations/               ← versioned SQL migration files
├── docker/
│   ├── Dockerfile            ← multi-stage build (builder + scratch runner)
│   └── docker-compose.yml
└── tests/
    ├── unit/                 ← rate limiter tests with miniredis
    └── integration/          ← full request lifecycle tests
```

---

## Interesting engineering decisions

**Sliding window rate limiter via Redis sorted sets**

Each API key has a sorted set in Redis where every member is a request UUID and its score is the timestamp in milliseconds. On each request, a Lua script atomically removes entries older than the window, counts what remains, and adds the new request if under the limit. The atomic Lua execution prevents race conditions that would allow clients to exceed their limit under concurrent load. A naive fixed-window counter allows clients to double their effective rate by timing requests at window boundaries — the sliding window prevents this.

**Two-stage AI analysis**

The worker doesn't call Claude on every request — that would be expensive and slow. A fast rule-based detector runs first (burst traffic > 40 req/min, error rate > 30%, scanning pattern > 20 unique IPs). Claude is only called when a rule triggers. Claude's job is specifically what LLMs are good at: synthesising ambiguous data into a plain-English explanation. Everything deterministic (counting, comparing, storing) is done in regular Go code.

**Two-binary design**

The gateway must be fast and synchronous. The AI analysis is slow (Claude API takes 1–3s) and fault-tolerant (if Claude is down, traffic still flows). Separating them into independent processes means gateway latency is never coupled to AI availability. Jobs are retried with exponential backoff via asynq, so no analysis is lost if the worker restarts.

**SHA-256 API key hashing**

Raw API keys are never stored. On creation, the raw key is returned to the caller once and only the SHA-256 hash is persisted. On every request, the incoming key is hashed and the hashes are compared. A database breach exposes hashes — computationally infeasible to reverse for a randomly-generated 64-character key.

**Repository pattern with interfaces**

Every database interaction is behind an interface. The auth middleware depends on `APIKeyRepository` (interface), not `PostgresAPIKeyRepo` (concrete). This means tests inject a mock that satisfies the same interface — no real database required for unit tests. This is Dependency Inversion from SOLID applied practically.

---

## Running locally

**Prerequisites:** Go 1.22+, Docker, Docker Compose, `golang-migrate` CLI

```bash
# 1. Clone and enter the project
git clone https://github.com/yourusername/ai-gateway
cd ai-gateway

# 2. Copy env file and fill in your values
cp .env.example .env
# Required: ANTHROPIC_API_KEY, JWT_SECRET, ADMIN_API_KEY

# 3. Start Postgres and Redis
docker compose -f docker/docker-compose.yml up postgres redis -d

# 4. Run migrations
migrate -path ./migrations \
  -database "postgres://gateway:secret@localhost:5432/gateway_db?sslmode=disable" up

# 5. Start the gateway
go run ./cmd/gateway

# 6. Start the AI worker (in a separate terminal)
go run ./cmd/worker
```

Gateway runs on `http://localhost:8080`.

---

## API reference

### System

```
GET  /health    → liveness check, no auth required
GET  /metrics   → Prometheus metrics
```

### Admin — API key management
All admin endpoints require `X-Admin-Key: <ADMIN_API_KEY>` header.

```
POST   /v1/admin/keys          → create a key (returns raw key once only)
GET    /v1/admin/keys          → list all keys
GET    /v1/admin/keys/:id      → get key details
PATCH  /v1/admin/keys/:id      → update rate limit, scopes, expiry
DELETE /v1/admin/keys/:id      → revoke key (soft delete)
GET    /v1/admin/keys/:id/stats → request count, error rate, p99 latency
```

### Admin — Route management

```
GET    /v1/admin/routes        → list all proxy routes
POST   /v1/admin/routes        → register a new upstream route
PUT    /v1/admin/routes/:id    → update route config
DELETE /v1/admin/routes/:id    → remove route
```

### Admin — Anomaly alerts

```
GET    /v1/admin/alerts                 → list alerts (filter: ?severity=HIGH&resolved=false)
GET    /v1/admin/alerts/:id             → get alert with full Claude explanation
PATCH  /v1/admin/alerts/:id/resolve     → resolve alert, re-enable auto-blocked key
```

### Gateway proxy

```
ANY  /api/*   → auth + rate-limited proxy to matched upstream
```

---

## Quick usage example

```bash
# Create an API key
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "X-Admin-Key: your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-service","owner_id":"user-1","rate_limit_rpm":60}'

# Register a route
curl -X POST http://localhost:8080/v1/admin/routes \
  -H "X-Admin-Key: your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"httpbin","path_pattern":"/api/test/*","target_url":"https://httpbin.org","strip_prefix":true}'

# Make a proxied request using the key
curl http://localhost:8080/api/test/get \
  -H "X-API-Key: <raw-key-from-create-response>"
```

---

## Running tests

```bash
# Unit tests (no external dependencies — uses miniredis)
go test ./tests/unit/... -v

# All tests with race condition detector
go test ./... -race

# Coverage report
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

---

## Production deployment

Build both binaries:

```bash
docker build --target gateway -t ai-gateway:latest .
docker build --target worker  -t ai-worker:latest  .
```

Or run the full stack with Docker Compose:

```bash
docker compose -f docker/docker-compose.yml up --build
```

This starts Postgres, Redis, the gateway, the worker, and the asynq monitoring UI at `http://localhost:8081`.

---

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `APP_PORT` | yes | Port the gateway listens on |
| `APP_ENV` | yes | `development` or `production` |
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `REDIS_URL` | yes | Redis connection string |
| `JWT_SECRET` | yes | Secret for signing/verifying JWTs |
| `ADMIN_API_KEY` | yes | Static key for admin endpoints |
| `ANTHROPIC_API_KEY` | yes | Claude API key |
| `ANTHROPIC_MODEL` | no | Default: `claude-sonnet-4-6` |
| `RATE_LIMIT_DEFAULT_RPM` | no | Default requests per minute (default: 60) |
| `RATE_LIMIT_WINDOW_SECONDS` | no | Window size in seconds (default: 60) |

---

## SE concepts demonstrated

- **Repository pattern** — all SQL isolated behind interfaces, injectable mocks in tests
- **Middleware / Chain of Responsibility** — each middleware owns exactly one concern
- **Producer-Consumer** — gateway enqueues, worker consumes, independently scalable
- **Strategy pattern** — rate limiter algorithm is interchangeable behind an interface
- **Circuit breaker** — CRITICAL alerts auto-disable keys until admin resolves
- **Sliding window algorithm** — atomic Redis Lua script, race-condition-safe
- **SOLID principles** — Single Responsibility (each middleware), Dependency Inversion (interfaces), Open/Closed (add routes via DB, not code changes)
- **Fail-fast startup** — missing env vars crash immediately with a clear message
- **Structured logging** — JSON via zap, every log line queryable
- **Observability** — Prometheus metrics, p99 latency histogram, request counter per route

---

## License

MIT
