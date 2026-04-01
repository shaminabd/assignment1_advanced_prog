# Order & Payment Microservices

Two microservices in Go: Order Service and Payment Service. Built with Clean Architecture, Gin framework, and PostgreSQL.

## Architecture Diagram

```
┌──────────────┐       POST /payments       ┌──────────────┐
│              │ ────────────────────────>   │              │
│ Order Service│                             │Payment Service│
│  (port 8083) │ <── Authorized / Declined   │  (port 8084) │
└──────┬───────┘                             └──────┬───────┘
       │                                            │
  ┌────▼─────┐                                 ┌────▼─────┐
  │ order_db │                                 │payment_db│
  │ (5432)   │                                 │ (5432)   │
  └──────────┘                                 └──────────┘
```

Each service has its own database. No shared tables or schemas.

## Project Structure

```
order-service/
├── cmd/order/main.go            - entry point
├── internal/
│   ├── app/app.go               - dependency setup and server start
│   ├── domain/order.go          - Order entity
│   ├── usecase/order.go         - business logic + interfaces
│   ├── repository/postgres.go   - database queries
│   ├── client/payment.go        - http client for payment service
│   └── transport/http/handler.go - gin handlers
└── migrations/

payment-service/
├── cmd/payment/main.go
├── internal/
│   ├── app/app.go
│   ├── domain/payment.go
│   ├── usecase/payment.go
│   ├── repository/postgres.go
│   └── transport/http/handler.go
└── migrations/
```

## Layers

- **Domain** - entity structs, no external dependencies
- **Use Case** - business logic, defines interfaces for repo and external clients
- **Repository** - implements data access with PostgreSQL
- **Transport** - HTTP handlers (Gin), only parses requests and returns responses
- **Client** (order-service only) - calls payment service over HTTP
- **App** - connects everything together, starts the server

Dependencies go inward: Transport -> Use Case -> Domain <- Repository

## Bounded Contexts

Order Service owns orders: creation, status updates, cancellation logic. It does not touch payment data directly, it calls Payment Service over HTTP.

Payment Service owns payments: authorization, transaction IDs, amount limits. It only knows about order_id, nothing else about orders.

Each service has its own Go module (`go.mod`), its own database, and its own models. No shared packages.

## Business Rules

- Money is `int64` (in cents), no floats
- Amount must be positive
- If payment amount > 100000, payment is declined
- Only pending orders can be cancelled, paid orders cannot

## Failure Handling

Order Service uses `http.Client` with a 2 second timeout when calling Payment Service.

If Payment Service is down or slow:
1. Timeout kicks in after 2 seconds
2. Order gets marked as "Failed"
3. Returns HTTP 503

I chose to mark the order as "Failed" instead of leaving it "Pending" because a pending order might confuse the user into thinking payment is still processing. Failed is more clear - the user can just try again.

## Idempotency (Bonus)

POST /orders accepts an `Idempotency-Key` header. If you send the same key twice, it returns the existing order instead of making a new one. This prevents duplicate orders if the client retries.

## How to Run (local Postgres)

I use local PostgreSQL on port 5432, without docker.

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

### 3. Start payment service

```bash
cd payment-service
go run cmd/payment/main.go
```

### 4. Start order service (new terminal)

```bash
cd order-service
go run cmd/order/main.go
```

## API Examples

Create order:
```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "cust-1", "item_name": "Laptop", "amount": 15000}'
```

Create order with idempotency:
```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-key-123" \
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

Get payment for order:
```bash
curl http://localhost:8084/payments/{order_id}
```

Test declined payment (over limit):
```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "cust-2", "item_name": "Expensive Item", "amount": 150000}'
```
