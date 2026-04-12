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
├── docs/
│   └── dashboard.html        ← single-file admin UI (open in browser)
├── docker/
│   ├── Dockerfile            ← multi-stage build (builder + scratch runner)
│   └── docker-compose.yml
└── tests/
    ├── unit/                 ← rate limiter tests with miniredis
    └── integration/          ← full request lifecycle tests
```

---

## Running locally — step by step

> **Prerequisites:** Go 1.22+, Docker Desktop

### 1. Clone and configure

```bash
git clone https://github.com/Maheesh09/AI-gateway
cd AI-gateway
```

Copy the example env file and fill in your values:

```bash
cp .env.example .env
```

Your `.env` should look like this:

```env
APP_PORT=8080
APP_ENV=development

DATABASE_URL=postgres://gateway:secret@127.0.0.1:5433/gateway_db?sslmode=disable
REDIS_URL=redis://127.0.0.1:6379

JWT_SECRET=your-random-secret-here
ADMIN_API_KEY=your-admin-key-here

ANTHROPIC_API_KEY=sk-ant-...
# Set to 'mock' to test the AI pipeline without spending credits:
ANTHROPIC_MODEL=mock

RATE_LIMIT_DEFAULT_RPM=60
RATE_LIMIT_WINDOW_SECONDS=60
```

> **Required values:** `ANTHROPIC_API_KEY`, `JWT_SECRET`, `ADMIN_API_KEY`

---

### 2. Start Postgres and Redis via Docker

```bash
docker compose -f docker/docker-compose.yml up postgres redis -d
```

Verify containers are running:

```bash
docker ps
# You should see: docker-postgres-1 and docker-redis-1
```

---

### 3. Set up the database

The Docker container creates the PostgreSQL user `gateway` but does **not** pre-create the database. Do it once with this command:

```powershell
docker exec -it docker-postgres-1 psql -U gateway -d postgres -c "CREATE DATABASE gateway_db;"
```

Then run the three migration files to create all tables:

```powershell
# Migration 1 — API keys table
Get-Content ./migrations/000001_create_api_keys.up.sql | docker exec -i docker-postgres-1 psql -U gateway -d gateway_db

# Migration 2 — Proxy routes table
Get-Content ./migrations/000002_create_proxy_routes.up.sql | docker exec -i docker-postgres-1 psql -U gateway -d gateway_db

# Migration 3 — Request logs and anomaly alerts tables
Get-Content ./migrations/000003_create_logs_alerts.up.sql | docker exec -i docker-postgres-1 psql -U gateway -d gateway_db
```

> `ERROR: relation "..." already exists` messages are safe to ignore — it means the tables already exist from a previous run.

---

### 4. Start the gateway and worker

Open **two separate terminal tabs** and run one command in each:

**Terminal 1 — Gateway (handles HTTP traffic):**
```bash
go run ./cmd/gateway
```

**Terminal 2 — AI Worker (processes background analysis jobs):**
```bash
go run ./cmd/worker
```

The gateway listens on `http://localhost:8080`. The worker connects to the same Redis queue and runs in the background — it has no HTTP port of its own.

---

### 5. Open the Admin Dashboard

Open `docs/dashboard.html` directly in your browser (no server needed):

```
d:\PROJECTS\ai-gateway\AI-gateway\docs\dashboard.html
```

On first load, go to **Settings** (bottom of the left sidebar) and enter:
- **Gateway URL:** `http://localhost:8080`
- **Admin API Key:** the value of `ADMIN_API_KEY` from your `.env`

Click **Save + Connect**. The status indicator in the bottom-left will turn green if the gateway is reachable.

---

## Using the system — walkthrough

This section explains the full flow from zero to a working proxied request.

### Concept: two types of key

| Key | Where used | What it does |
|---|---|---|
| `ADMIN_API_KEY` | `X-Admin-Key` header | Unlocks `/v1/admin/*` endpoints to manage the system |
| Client API Key | `X-API-Key` header | Used by end-clients to make proxied requests through `/api/*` |

---

### Step 1 — Create a client API key

**Via the Dashboard:** Go to **API Keys → Create Key**, fill in the name and owner, click Create. Copy the raw key shown — it will not be displayed again.

**Via PowerShell:**
```powershell
# Define your admin headers (run this first in every new terminal session)
$adminHeaders = @{
    "X-Admin-Key"  = "your-ADMIN_API_KEY-from-env"
    "Content-Type" = "application/json"
}

$keyBody = @{
    name          = "my-test-service"
    owner_id      = "user-123"
    rate_limit_rpm = 60
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/keys" -Method POST -Headers $adminHeaders -Body $keyBody
$response | ConvertTo-Json

# Save the raw_key value — you will need it in Step 3
```

---

### Step 2 — Register a proxy route

This tells the gateway: *"Requests to `/api/fakestore/*` should be forwarded to `https://fakestoreapi.com`".*

**Via the Dashboard:** Go to **Proxy Routes → Add Route** and fill in the form.

**Via PowerShell:**
```powershell
$routeBody = @{
    name         = "httpbin"
    path_pattern = "/api/httpbin/*"
    target_url   = "https://httpbin.org"
    strip_prefix = $true
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/routes" -Method POST -Headers $adminHeaders -Body $routeBody
```

> **`strip_prefix = true`** means the gateway strips `/api/httpbin` before forwarding, so `/api/httpbin/get` becomes `/get` at the upstream.

> **Note on FakeStoreAPI:** `fakestoreapi.com` has recurring SSL issues. Use `https://httpbin.org` as a reliable free alternative during development.

---

### Step 3 — Send a proxied request

Replace `<YOUR_RAW_KEY>` with the key from Step 1.

**Via the Dashboard:** Go to **Test Proxy**, paste your API key, set the path to `/api/httpbin/get`, and click Send.

**Via PowerShell:**
```powershell
$clientHeaders = @{
    "X-API-Key" = "<YOUR_RAW_KEY>"
}

$response = Invoke-RestMethod -Uri "http://localhost:8080/api/httpbin/get" -Method GET -Headers $clientHeaders
$response | ConvertTo-Json
```

> **Important:** The `X-API-Key` header must contain **only** the raw `gw_...` key string — not the full JSON object returned by the create endpoint.

The request passes through the full middleware chain:
1. Auth middleware validates your API key against the database
2. Rate limiter checks you haven't exceeded 60 req/min (Lua script, Redis sorted set)
3. Route matcher finds the `fakestore` route
4. Reverse proxy forwards to `https://fakestoreapi.com/products/1`
5. Async job is enqueued for AI analysis

---

### Step 4 — Trigger the AI anomaly detector

Send 50 rapid requests to trigger the burst-traffic rule (threshold: 40 req/min):

> **Before running:** Set `$clientHeaders` in your PowerShell session:
> ```powershell
> $clientHeaders = @{ "X-API-Key" = "<YOUR_RAW_KEY>" }
> ```

```powershell
1..50 | ForEach-Object {
    Invoke-RestMethod -Uri "http://localhost:8080/api/httpbin/get" -Method GET -Headers $clientHeaders
    Write-Host "Sent request $_"
}
```

Watch the **worker terminal** — you will see log lines like:
```
[detector] stats for key ...: total=XX, rpm=YY (threshold=40)
[worker] !! ANOMALY DETECTED: type=burst_traffic, ...
[worker] AI Analysis SUCCESS: severity=HIGH, ...
```

> **No Anthropic credits?** Set `ANTHROPIC_MODEL=mock` in your `.env` file. The mock analyzer produces realistic canned responses for each trigger type (`burst_traffic`, `error_spike`, `scan_pattern`) without making any external API calls.

---

### Step 5 — View AI alerts

**Via the Dashboard:** Go to **AI Alerts** to see all alerts. Click **Details** on any alert to read Claude's full explanation.

**Via PowerShell:**
```powershell
$alerts = Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/alerts" -Method GET -Headers $adminHeaders
$alerts | ConvertTo-Json -Depth 5
```

To resolve an alert and re-enable a blocked key:
```powershell
$alertId = "<alert-id-from-above>"
Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/alerts/$alertId/resolve?re_enable_key=true" -Method PATCH -Headers $adminHeaders
```

---

## Admin Dashboard

The dashboard (`docs/dashboard.html`) is a zero-dependency, single HTML file that connects directly to the running gateway via the browser's Fetch API. No build step, no Node.js, no server required — just open it in a browser.

| Page | What you can do |
|---|---|
| **Overview** | See live stats (key count, route count, open alerts, health) and the full request pipeline diagram |
| **API Keys** | Create keys (raw key shown once), edit rate limits, toggle active/inactive, revoke |
| **Proxy Routes** | Add routes with path patterns and target URLs, configure strip-prefix and timeout, delete routes |
| **Test Proxy** | Send a real proxied request using an API key and see the full response with status and latency |
| **AI Alerts** | Filter by severity and resolved state, read Claude's plain-English explanations, resolve alerts and optionally re-enable disabled keys |
| **Settings** | Configure the gateway URL and admin key (stored in browser localStorage) |

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
POST   /v1/admin/keys              → create a key (returns raw key once only)
GET    /v1/admin/keys              → list all keys
GET    /v1/admin/keys/:id          → get key details
PATCH  /v1/admin/keys/:id          → update rate_limit_rpm or is_active
DELETE /v1/admin/keys/:id          → revoke key (soft delete / sets is_active=false)
GET    /v1/admin/keys/:id/stats    → request count, error rate, p99 latency
```

### Admin — Route management

```
GET    /v1/admin/routes            → list all proxy routes
POST   /v1/admin/routes            → register a new upstream route
PUT    /v1/admin/routes/:id        → update route config
DELETE /v1/admin/routes/:id        → remove route
```

### Admin — Anomaly alerts

```
GET    /v1/admin/alerts                       → list alerts (?severity=HIGH&resolved=false)
GET    /v1/admin/alerts/:id                   → get alert with full Claude explanation
PATCH  /v1/admin/alerts/:id/resolve           → resolve alert (?re_enable_key=true to also re-enable key)
```

### Gateway proxy

```
ANY  /api/*   → auth + rate-limited proxy to matched upstream
```

---

## PowerShell reference (Windows)

Because `curl` in PowerShell uses different syntax than bash, all API calls should use `Invoke-RestMethod`. Key rule: **always define `$adminHeaders` at the start of each terminal session.**

```powershell
# ── Setup (run once at the start of each terminal session) ──────────────
$adminHeaders = @{
    "X-Admin-Key"  = "your-ADMIN_API_KEY"
    "Content-Type" = "application/json"
}

# ── Health check ──────────────────────────────────────────────────────────
Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET

# ── Create an API key ─────────────────────────────────────────────────────
Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/keys" -Method POST `
    -Headers $adminHeaders `
    -Body (@{ name="my-svc"; owner_id="user-1"; rate_limit_rpm=60 } | ConvertTo-Json)

# ── List all keys ─────────────────────────────────────────────────────────
Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/keys" -Method GET -Headers $adminHeaders

# ── Add a proxy route ─────────────────────────────────────────────────────
Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/routes" -Method POST `
    -Headers $adminHeaders `
    -Body (@{ name="fakestore"; path_pattern="/api/fakestore/*"; target_url="https://fakestoreapi.com"; strip_prefix=$true } | ConvertTo-Json)

# ── Send a proxied request (use the raw client key, NOT the admin key) ────
$clientHeaders = @{ "X-API-Key" = "<raw-key-from-create-response>" }
Invoke-RestMethod -Uri "http://localhost:8080/api/fakestore/products/1" -Method GET -Headers $clientHeaders

# ── List alerts ───────────────────────────────────────────────────────────
Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/alerts?resolved=false" -Method GET -Headers $adminHeaders

# ── Resolve an alert ──────────────────────────────────────────────────────
Invoke-RestMethod -Uri "http://localhost:8080/v1/admin/alerts/<id>/resolve?re_enable_key=true" -Method PATCH -Headers $adminHeaders
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
| `ANTHROPIC_MODEL` | no | Model name or `mock` (default: `claude-3-5-sonnet-20240620`). Set to `mock` to test the AI pipeline without API credits. |
| `RATE_LIMIT_DEFAULT_RPM` | no | Default requests per minute (default: 60) |
| `RATE_LIMIT_WINDOW_SECONDS` | no | Window size in seconds (default: 60) |

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
