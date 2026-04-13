# Order & Payment Microservices

Repo: `https://github.com/shaminabd/assignment1_advanced_prog`

Two microservices in Go: Order Service and Payment Service. Built with Clean Architecture, Gin (REST for external API), gRPC between services (Assignment 2), and PostgreSQL.

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
| `DATABASE_URL` | both | Postgres DSN | see `internal/app/app.go` |
| `PORT` | both | Gin HTTP port | order `8083`, payment `8084` |
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
│   ├── transport/http/
│   └── transport/grpc/          - ProcessPayment + logging interceptor
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
  -d '{"customer_id": "cust-1", "item_name": "Laptop", "amount": 15000}'
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
