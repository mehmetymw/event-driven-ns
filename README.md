# Event-Driven Notification System

A **high-throughput, low-latency** notification backend built in **Go**. It processes and delivers messages across SMS, Email, and Push using **Apache Kafka** for durable, partition-level parallelism and **PostgreSQL** for consistency — with **observability** (tracing, metrics, structured logs) built in so distributed flows are easy to monitor and debug in production.

## Why This Stack

- **Go** — API and Worker run as separate binaries for independent horizontal scaling and clear concurrency boundaries.
- **Kafka (KRaft)** — Priority-based topics, durable log, and consumer groups so the system can handle burst traffic and millions of messages per day without dropping work.
- **PostgreSQL** — Source of truth for notifications, batches, idempotency, and templates; no Redis dependency for core consistency.
- **Hexagonal architecture** — Domain and application logic stay free of infrastructure; Kafka, HTTP, and DB are pluggable adapters. Easy to test and evolve.
- **Observability** — Distributed tracing (OpenTelemetry → Jaeger), trace–log correlation, per-channel metrics, and structured JSON logs so every request and delivery can be followed across API, Kafka, and worker.

## Architecture

Two processes share the same codebase: **API** (synchronous HTTP) and **Worker** (Kafka consumer + scheduler). Both talk to PostgreSQL and Kafka; the domain and application layers are identical, only the adapters (HTTP vs consumer loop) differ. This keeps scaling and deployment simple: scale API and Worker independently.

**Components**

```
                    ┌─────────────┐
                    │   Client    │
                    └──────┬──────┘
                           │ HTTP
         ┌─────────────────▼─────────────────┐     ┌──────────────┐
         │            API (Go binary)         │────▶│  PostgreSQL  │
         │  Handlers · Middleware · Services  │◀────│              │
         └─────────────────┬─────────────────┘     └──────────────┘
                           │ produce                    ▲
                           ▼                            │
                    ┌─────────────┐                     │
                    │    Kafka    │  notifications.*    │
                    │  (3 topics) │                     │
                    └──────┬──────┘                     │
                           │ consume                    │
         ┌─────────────────▼─────────────────┐         │
         │         Worker (Go binary)         │─────────┘
         │  Consumer · Scheduler · Delivery   │
         └─────────────────┬─────────────────┘
                           │ HTTP
                    ┌──────▼──────┐
                    │  Provider   │  (e.g. webhook.site)
                    └─────────────┘
```

**Layers (hexagonal)** — Domain and app logic depend only on interfaces (ports); infrastructure implements them (adapters).

```
  cmd/api, cmd/worker
         │
  ┌──────▼──────┐
  │  Adapters   │  HTTP · Kafka · Postgres · Provider · WebSocket
  └──────┬──────┘
  ┌──────▼──────┐
  │    App      │  NotificationService · DeliveryService · Scheduler
  └──────┬──────┘
  ┌──────▼──────┐
  │   Ports     │  Repository · QueuePublisher/Consumer · DeliveryProvider
  └──────┬──────┘
  ┌──────▼──────┐
  │   Domain    │  Notification · Template · Validation · Errors
  └─────────────┘
```

**Request flow (create → deliver)**

```
Client → API → Validate → Persist (PostgreSQL) → Produce (Kafka)
                                                       │
                 ┌─────────────────────────────────────┘
                 ▼
Worker: Consume → Rate limit → Circuit breaker → Provider (HTTP)
                 │                                      │
          ┌──────┴──────┐                        ┌──────┴──────┐
          │  Transient  │                        │   Success   │
          │  error?     │                        │  Update DB  │
          │  Re-produce │                        │  Broadcast  │
          │  to Kafka   │                        │  via WS     │
          └─────────────┘                        └─────────────┘
```

## Tech Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Language | Go | 1.25 |
| Database | PostgreSQL | 18 |
| Message broker | Apache Kafka (KRaft) | 4.2 |
| HTTP | gin-gonic/gin | 1.11 |
| DB access | jmoiron/sqlx + jackc/pgx | v5.8 |
| Tracing | OpenTelemetry + Jaeger | OTel 1.40 |
| Circuit breaker | sony/gobreaker | v2.4 |
| WebSocket | coder/websocket | 1.8 |
| Testing | stretchr/testify | 1.11 |

## Quick Start

```bash
git clone https://github.com/mehmetymw/event-driven-ns.git
cd event-driven-ns

cp .env.example .env
# Set WEBHOOK_URL=https://webhook.site/YOUR-UUID

docker compose up --build -d
```

- **API:** `http://localhost:8080`
- **Jaeger UI:** `http://localhost:16686`
- **Swagger UI:** `http://localhost:8080/swagger/`

## Observability

Built for **monitoring and debugging** in distributed, high-throughput setups:

| Layer | What’s in place |
|-------|------------------|
| **Distributed tracing** | OpenTelemetry SDK with OTLP export to Jaeger. Spans from API (otelgin), worker, Kafka consume/send, and outbound webhook (otelhttp). Trace context propagated so one trace covers create → queue → deliver. |
| **Logs** | Structured JSON (Zap). Every log line can carry `trace_id` and `span_id` for correlation with Jaeger. Correlation ID on HTTP requests for request-scoped debugging. |
| **Metrics** | `/api/v1/metrics` — per-channel sent/failed counts, average latency, success rate. DB-backed so API and worker share the same view. |
| **Health** | `/health` (liveness), `/health/ready` (PostgreSQL + Kafka reachability) for orchestration and load balancers. |
| **Errors in traces** | Spans record errors and set status so failures are visible in Jaeger without digging through logs. |

One flow in Jaeger can show: HTTP request → notification create → Kafka produce → (worker) consume → provider call → DB update.

## API Overview

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/notifications` | Create notification |
| `POST` | `/api/v1/notifications/batch` | Create batch (up to 1000) |
| `GET` | `/api/v1/notifications/:id` | Get by ID |
| `GET` | `/api/v1/notifications` | List with filters + pagination |
| `PATCH` | `/api/v1/notifications/:id/cancel` | Cancel pending |
| `GET` | `/api/v1/batches/:id` | Batch status |
| `POST` | `/api/v1/templates` | Create template |
| `GET` | `/api/v1/templates` | List templates |
| `GET` | `/api/v1/metrics` | Per-channel metrics |
| `GET` | `/health` | Liveness |
| `GET` | `/health/ready` | Readiness (DB + Kafka) |
| `GET` | `/ws` | WebSocket status updates |

## Reliability & Scale

- **Retry:** Exponential backoff with jitter; max retries by priority (High=5, Normal=3, Low=2). Transient errors (timeout, 5xx) re-produced to Kafka.
- **Circuit breaker:** Per-channel (gobreaker); opens after 5 failures, half-open after 30s to avoid cascading failures.
- **Rate limiting:** 100 msg/sec per channel (token bucket) in the worker so external providers are not overloaded.
- **Idempotency:** PostgreSQL-backed; duplicate keys return the existing notification with 409.

## Testing

```bash
go test -v -race -count=1 ./...
```

Unit tests cover domain validation, application services (with mocks), HTTP binding, and scheduler behavior. For a full flow against a running stack:

```bash
./scripts/test.sh
```

## Project Structure

```
├── cmd/
│   ├── api/main.go              HTTP API binary
│   └── worker/main.go           Kafka consumer + scheduler binary
├── internal/
│   ├── domain/                  Entities, validation, errors
│   ├── port/                    Interfaces (repository, queue, provider)
│   ├── app/                     Application services
│   └── adapter/
│       ├── http/                Gin handlers, middleware, DTOs
│       ├── postgres/            Repositories
│       ├── queue/               Kafka producer & consumer
│       ├── provider/            Webhook client + circuit breaker
│       └── ws/                  WebSocket hub
├── pkg/                         Config, logger, tracing, circuitbreaker
├── migrations/                  Versioned SQL (golang-migrate)
├── scripts/                     E2E and channel test scripts
├── docs/                        OpenAPI spec + Swagger UI
├── Dockerfile                   Multi-stage (api + worker)
└── docker-compose.yaml          API, Worker, PostgreSQL, Kafka, Jaeger
```
