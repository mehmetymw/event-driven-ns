# Event-Driven Notification System

A scalable, event-driven notification system built with Go that processes and delivers messages through SMS, Email, and Push channels using Apache Kafka.

## Architecture

Hexagonal architecture (ports & adapters) with API and Worker as separate binaries for independent scaling.

```
Client → API → Validate → Persist (PostgreSQL) → Produce (Kafka)
                                                       │
                 ┌─────────────────────────────────────┘
                 ↓
Kafka Consumer (Worker) → Rate Limit → Circuit Breaker → webhook.site
                 ↓                                            │
          ┌──────┴──────┐                              ┌──────┴──────┐
          │  Transient   │                              │   Success   │
          │   Error?     │                              │             │
          │  → re-produce│                              │ Update DB   │
          │  to Kafka    │                              │ Broadcast WS│
          └──────────────┘                              └─────────────┘
```

## Tech Stack

| Component | Technology | Version |
|---|---|---|
| Language | Go | 1.25 |
| Database | PostgreSQL | 18 |
| Message Broker | Apache Kafka (KRaft) | 4.2 |
| HTTP Framework | gin-gonic/gin | 1.11 |
| DB Access | jmoiron/sqlx + jackc/pgx | v5.8 |
| Distributed Tracing | OpenTelemetry + Jaeger | OTel 1.40 |
| Circuit Breaker | sony/gobreaker | v2.4 |
| WebSocket | coder/websocket | 1.8 |
| Testing | stretchr/testify | 1.11 |

## Quick Start

```bash
git clone https://github.com/mehmetymw/event-driven-ns.git
cd event-driven-ns

cp .env.example .env
# Set WEBHOOK_URL=https://webhook.site/YOUR-UUID

docker-compose up --build -d
```

- API: `http://localhost:8080`
- Jaeger UI: `http://localhost:16686`
- Swagger UI: `http://localhost:8080/swagger/`

## Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/notifications` | Create notification |
| `POST` | `/api/v1/notifications/batch` | Create batch (up to 1000) |
| `GET` | `/api/v1/notifications/:id` | Get by ID |
| `GET` | `/api/v1/notifications` | List with filters + pagination |
| `PATCH` | `/api/v1/notifications/:id/cancel` | Cancel pending |
| `GET` | `/api/v1/batches/:id` | Batch status |
| `POST` | `/api/v1/templates` | Create template |
| `GET` | `/api/v1/templates` | List templates |
| `GET` | `/api/v1/metrics` | Real-time metrics |
| `GET` | `/health` | Liveness probe |
| `GET` | `/health/ready` | Readiness probe |
| `GET` | `/ws` | WebSocket status updates |

## Delivery & Retry Strategy

- **Exponential backoff**: `min(1s * 2^attempt + jitter, 5min)`
- **Max retries by priority**: High=5, Normal=3, Low=2
- **Transient errors** (retry): timeout, 429, 5xx
- **Permanent errors** (fail): 400, 401, 403, 404
- **Circuit breaker**: Opens after 5 consecutive failures, half-open after 30s
- **Rate limiting**: 100 msg/sec per channel via token bucket

## Testing

### Unit Tests

```bash
go test -v -race -count=1 ./...
```

Tests are colocated with source code following Go conventions. Covers domain validation, application service orchestration (mock-based), HTTP handler binding, scheduler logic.

### Integration Test Script

```bash
./scripts/test.sh
```

End-to-end test against a running instance. Creates templates, sends notifications (single + batch), checks delivery status, and verifies metrics.

## Project Structure

```
├── cmd/
│   ├── api/main.go              HTTP API binary
│   └── worker/main.go           Kafka consumer + scheduler binary
├── internal/
│   ├── domain/                  Entities, validation, errors
│   ├── port/                    Interface definitions
│   ├── app/                     Application services
│   └── adapter/
│       ├── http/                Gin handlers, middleware, DTOs
│       ├── postgres/            sqlx repositories
│       ├── queue/               Kafka producer & consumer
│       ├── provider/            Webhook client + circuit breaker
│       └── ws/                  WebSocket hub
├── pkg/
│   ├── config/                  Environment-based config
│   ├── logger/                  Zap logger + correlation ID
│   ├── tracing/                 OpenTelemetry initialization
│   └── circuitbreaker/          gobreaker wrapper
├── migrations/                  Versioned SQL migrations
├── scripts/                     Test and utility scripts
├── docs/                        OpenAPI spec + Swagger UI
├── Dockerfile                   Multi-stage build (api + worker)
└── docker-compose.yaml          Full stack orchestration
```
