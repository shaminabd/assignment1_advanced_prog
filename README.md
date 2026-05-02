# Order & Payment Microservices

Repo: `https://github.com/shaminabd/assignment1_advanced_prog`

Three microservices in Go: **Order**, **Payment**, and **Notification**. Assignment 2 synchronous flow (REST → gRPC) is extended in Assignment 3 with **event-driven** notifications via **RabbitMQ** (producer in Payment, consumer in Notification), PostgreSQL for each bounded context, **manual message acknowledgements**, durable queues/messages, **idempotent consumption** by `event_id`, and **graceful shutdown** (`SIGINT`/`SIGTERM`).

## Assignment 3 — Event-driven notifications (RabbitMQ)

### Architecture diagram (template)

- Editable Mermaid source: [`docs/assignment3-architecture.mmd`](docs/assignment3-architecture.mmd) — open in [Mermaid Live Editor](https://mermaid.live) and export **PNG/SVG** for the submission PDF/report.

### Reliability and ACKs

- **Publisher (Payment):** after `payments` row insert succeeds and status is `Authorized`, Payment publishes a JSON event (`event_id`, `order_id`, `amount`, `customer_email`, `status`) to the durable exchange `payment.events` with routing key `payment.completed`. Messages use **persistent** delivery; the publisher enables RabbitMQ **publisher confirms** and treats the operation as failed if the broker does not acknowledge the publish.
- **Consumer (Notification):** `Consume` uses **`autoAck=false`**. A message is **`Ack`nowledged only after** the email line is logged to stdout (and the idempotency row is committed). If DB insert/logging fails before ack, the message stays in the queue or can be **nack-requeued** for transient errors.

### Idempotency strategy

- Each event carries a unique **`event_id`** (UUID) generated at publish time.
- Notification stores processed IDs in Postgres table **`processed_events(event_id UUID PRIMARY KEY)`**.
- On delivery, the consumer runs **`INSERT … ON CONFLICT (event_id) DO NOTHING`**. If **no row was inserted**, the message is a duplicate: **ack without logging again**. If inserted, log once then ack.

### Docker Compose (full stack)

From the repository root:

```bash
docker compose up --build
```

Services: **postgres** (creates `order_db`, `payment_db`, `notification_db` and core tables via [`deploy/postgres/docker-init.sh`](deploy/postgres/docker-init.sh)), **rabbitmq** (AMQP `5672`, management UI `15672`), **payment-service**, **order-service**, **notification-service**.

Environment highlights:

| Variable | Service | Purpose |
|----------|---------|---------|
| `RABBITMQ_URL` | payment, notification | AMQP URI (compose: `amqp://guest:guest@rabbitmq:5672/`) |
| `DATABASE_URL` | all three | Per-service database (see compose file) |
| `PAYMENT_GRPC_ADDR` | order | Payment address (`payment-service:50052` in compose) |

**Fresh DB volume:** if Postgres data already exists without the new databases, run `docker compose down -v` once before `up` so `docker-init.sh` runs again.

**Postgres host port:** the compose file maps Postgres to **`localhost:5433`** → container `5432`, so it does not conflict with a **local PostgreSQL** often bound to **`5432`**. Services inside Docker still use `postgres:5432`. To connect from your shell with `psql`, use port **5433**.

If you prefer port 5432 on the host instead, stop the other Postgres (`brew services stop postgresql@…`, quit Postgres.app, etc.) and change the mapping in [`docker-compose.yml`](docker-compose.yml) back to `"5432:5432"`.

### Demo API (with email for notifications)

```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-1","item_name":"Laptop","amount":9999,"customer_email":"user@example.com"}'
```

Payment publishes only when the payment status is **Authorized** (business rule: amounts above **100000** cents are declined — adjust `amount` for a successful notification).

Watch Notification logs:

```bash
docker compose logs -f notification-service
```

Expected line format:

```text
[Notification] Sent email to user@example.com for Order #<order_id>. Amount: $99.99
```

### Messaging layout (RabbitMQ)

- Exchange: `payment.events` (direct, durable)
- Queue: `payment.completed` (durable), bound with routing key `payment.completed`
- Dead-letter exchange: `payment.dlx` (direct, durable); dead-letter queue: `payment.completed.dlq` bound with routing key `payment.completed.dlq`
- Main queue declares **`x-dead-letter-exchange`** / **`x-dead-letter-routing-key`** so that **`basic.nack` / `reject` with `requeue=false`** routes the message to the DLQ (after retries in the poison demo).
- JSON payload fields: `event_id`, `order_id`, `amount` (cents), `customer_email`, `status`

### Bonus (+10%): Dead Letter Queue (DLQ) demo

- **Topology:** Payment publisher and Notification consumer both declare the same DLX/DLQ bindings so failed poison messages are routed to **`payment.completed.dlq`**.
- **Poison trigger:** set `"customer_email":"poison@dlq.demo"` on `POST /orders` (amount must stay ≤ **100000** cents so the payment is **Authorized** and an event is published).
- **Retries:** the consumer simulates a hard failure for that email. Deliveries **1–2** use **`Nack(requeue=true)`** (retry). On delivery **3**, it **`Nack(requeue=false)`**, RabbitMQ dead-letters the message to **`payment.completed.dlq`**.
- **Observation:** the service runs a second consumer on the DLQ only to log lines prefixed **`[DLQ]`** so you can demonstrate the move in `docker compose logs`.
- **Important:** if RabbitMQ already had an older `payment.completed` queue **without** DLX arguments, declaration fails (`PRECONDITION_FAILED`). Reset broker state with **`docker compose down -v`** (or delete the old queue in the management UI at http://localhost:15672) before `up --build`.

```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"dlq-demo","item_name":"Poison","amount":5000,"customer_email":"poison@dlq.demo"}'
```

Then watch logs:

```bash
docker compose logs -f notification-service
```

You should see three `poison demo — simulated failure` lines (attempts 1–3), then **`poison demo — max retries reached, rejecting to DLQ`**, then **`[DLQ] message moved to dead-letter queue …`**.

## Assignment 2 — Contract-first gRPC

### Proto repository (Repository A)

Source `.proto` files live in this monorepo under [`proto/`](proto/). For submission, push that tree to a **dedicated** GitHub repo that contains only contracts.

Suggested URL (replace with yours): `https://github.com/shaminabd/ap2-protos`

### Generated Go repository (Repository B)

Generated `*.pb.go` / `*_grpc.pb.go` live in [`rpcgen/`](rpcgen/) as module `github.com/shaminabd/ap2-contracts-go`. For full marks, push `rpcgen` to its own repo, tag releases (e.g. `v1.0.0`), and point `go.mod` to `go get ...@v1.0.0` instead of `replace`.

Suggested URL (replace with yours): `https://github.com/shaminabd/ap2-contracts-go`

Local monorepo builds use:

```go
replace github.com/shaminabd/ap2-contracts-go => ../rpcgen
```

Regenerate after editing protos (requires `protoc` and `protoc-gen-go` / `protoc-gen-go-grpc` on `PATH`):

```bash
./scripts/generate-protobuf.sh
```

CI checks that generated files match protos: [`.github/workflows/protobuf.yml`](.github/workflows/protobuf.yml).

Remote-generation template for Repository B: [`rpcgen/.github/workflows/generate-from-proto-repo.yml`](rpcgen/.github/workflows/generate-from-proto-repo.yml) (optional secret `GH_PROTO_READ_TOKEN` if Repository A is private).

## Architecture Diagram (Assignment 2)

```
                    REST (Gin)                     gRPC ProcessPayment
              ┌──────────────────┐   ┌────────────────────────────────────┐
  Client ──── │   Order Service  │───│         Payment Service            │
              │  HTTP :8083      │   │  gRPC PAYMENT_GRPC_LISTEN (:50052) │
              │  gRPC :50051     │   │  HTTP :8084 (optional)             │
              └────────┬─────────┘   └─────────────────┬──────────────────┘
                       │                               │
              gRPC SubscribeToOrderUpdates             │
              (stream, demo client)                    │
                       │                               │
                  ┌────▼─────┐                   ┌─────▼──────┐
                  │ order_db │                   │ payment_db │
                  └──────────┘                   └────────────┘
```

- End users call Order over **REST**. Order calls Payment over **gRPC** (`ProcessPayment`).
- Order exposes **server-streaming** `SubscribeToOrderUpdates` on the gRPC port; subscribers receive updates when order `status` changes in the database (after successful `UPDATE`).

## Environment variables

| Variable | Service | Purpose | Default (local) |
|----------|---------|---------|-----------------|
| `DATABASE_URL` | order / payment / notification | Postgres DSN | see each `internal/app/app.go` |
| `RABBITMQ_URL` | payment, notification | RabbitMQ AMQP URL | unset payment skips publish; notification defaults to localhost guest |
| `PORT` | order, payment | Gin HTTP port | order `8083`, payment `8084` |
| `PAYMENT_GRPC_ADDR` | order | Dial address for Payment gRPC | `localhost:50052` |
| `PAYMENT_GRPC_LISTEN` | payment | Payment gRPC listen address | `:50052` |
| `ORDER_GRPC_LISTEN` | order | Order gRPC listen (streaming) | `:50051` |
| `ORDER_GRPC_ADDR` | subscribe CLI | Order gRPC dial address | `localhost:50051` |

Do not hardcode deployment IPs in source; use env or `.env` loaded by your shell.

## Project Structure

```
proto/                           - Repository A (contracts)
rpcgen/                          - Repository B (generated Go module)
scripts/generate-protobuf.sh     - local codegen
order-service/
├── cmd/order/main.go
├── cmd/subscribe/main.go        - streaming demo client
├── internal/
│   ├── app/app.go
│   ├── client/payment.go        - gRPC client → Payment
│   ├── streaming/hub.go         - order status fan-out
│   ├── transport/http/          - Gin
│   └── transport/grpc/          - SubscribeToOrderUpdates
payment-service/
├── internal/
│   ├── infrastructure/rabbitmq/ - publisher confirms + durable topology
│   ├── messaging/               - event contract + publisher interface
│   ├── transport/http/
│   └── transport/grpc/          - ProcessPayment + logging interceptor
notification-service/
├── cmd/notification/main.go
└── internal/
    ├── infrastructure/rabbitmq/ - consumer, manual ack, QoS
    └── repository/               - processed_events idempotency
```

## Layers

- **Domain** — entities
- **Use Case** — rules; `PaymentClient` interface implemented by gRPC client
- **Repository** — Postgres; `UpdateStatus` notifies the streaming hub after a successful write
- **Transport** — Gin (REST) and gRPC servers (thin: map to use case / stream)
- **App** — wiring, env, starts HTTP + gRPC

## Bounded Contexts

Order Service owns orders and calls Payment only via gRPC for authorization. Payment Service owns payments and persists them in `payment_db`.

## Business Rules

- Money is `int64` (cents)
- Amount must be positive
- If payment amount > 100000, payment is declined
- Only pending orders can be cancelled; paid orders cannot

## Failure Handling

Order Service uses a **5s** timeout per gRPC call to Payment. If Payment is unavailable or the call times out, the order is marked **Failed** and REST returns **503** with `payment service is not available`.

## Idempotency

`POST /orders` accepts `Idempotency-Key`; duplicate keys return the existing order.

## How to Run (local Postgres)

### 1. Create databases

```bash
psql "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" -c "CREATE DATABASE order_db;"
psql "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" -c "CREATE DATABASE payment_db;"
```

### 2. Run migrations

```bash
psql "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable" -f order-service/migrations/001_create_orders.sql

psql "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable" -f payment-service/migrations/001_create_payments.sql
```

### 3. Start payment service (gRPC + HTTP)

```bash
cd payment-service
go run cmd/payment/main.go
```

### 4. Start order service (REST + gRPC streaming server)

```bash
cd order-service
go run cmd/order/main.go
```

### 5. Subscribe to order updates (separate terminal)

```bash
cd order-service
go run ./cmd/subscribe <order_id>
```

Create an order with `curl`, copy the returned `id`, run `subscribe` with that id, then trigger status changes (e.g. another `curl` create/cancel on a different flow). You should see **Paid**, **Failed**, or **Cancelled** events on the stream after the database row updates.

## API Examples

Create order:

```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-1","item_name":"Laptop","amount":15000,"customer_email":"user@example.com"}'
```

Get order:

```bash
curl http://localhost:8083/orders/{order_id}
```

Cancel order:

```bash
curl -X PATCH http://localhost:8083/orders/{order_id}/cancel
```

Get payment (HTTP on Payment service):

```bash
curl http://localhost:8084/payments/{order_id}
```

## Submission / evidence

- ZIP per course naming, upload to Moodle.
- Include **screenshots**: successful `ProcessPayment` (e.g. payment logs / grpcurl) and **streaming** output when DB status changes.
- Defense: clone from LMS, run services + subscriber, answer questions.

## Bonus (+10%)

Payment gRPC **unary interceptor** logs each method and duration: [`payment-service/internal/transport/grpc/logging_interceptor.go`](payment-service/internal/transport/grpc/logging_interceptor.go).
