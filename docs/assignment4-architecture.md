# Assignment 4 Architecture

## Architecture Diagram (Assignment 4)

```text
                          REST (Gin)                    gRPC ProcessPayment
                    ┌──────────────────┐      ┌────────────────────────────────────┐
  Client ──────────►│   Order Service  │─────►│         Payment Service            │
                    │    HTTP :8083    │      │   gRPC :50052 / HTTP :8084        │
                    └────────┬─────────┘      └─────────────────┬──────────────────┘
                             │                                  │
              cache-aside    │ write order                      │ write payment
              GET /orders/:id│                                  │
                             ▼                                  ▼
                       ┌────────────┐                     ┌─────────────┐
                       │  order_db  │                     │ payment_db  │
                       └────────────┘                     └─────────────┘
                             ▲                                  │
                             │ invalidate on UpdateStatus       │ publish event
                             │                                  ▼
                    ┌────────┴─────────┐              ┌────────────────────────────┐
                    │      Redis       │              │ RabbitMQ                   │
                    │ order:{id} TTL   │              │ payment.completed queue    │
                    │ ratelimit:ip:*   │              └──────────────┬─────────────┘
                    │ notification:sent│                             │ consume
                    └────────┬─────────┘                             ▼
                             │                            ┌──────────────────────────┐
                             │ idempotency + retries    │ Notification Service     │
                             └──────────────────────────│ EmailSender adapter      │
                                                          │ SIMULATED or SMTP REAL   │
                                                          └─────────────┬────────────┘
                                                                        │
                                                                        ▼
                                                          ┌─────────────────────────┐
                                                          │ provider send / log     │
                                                          └─────────────────────────┘
```

## Cache-aside flow

1. `GET /orders/:id` reads Redis key `order:{id}`.
2. On miss, Order Service loads the row from Postgres and stores JSON in Redis with `ORDER_CACHE_TTL`.
3. When status changes (`Paid`, `Failed`, `Cancelled`), the use case deletes `order:{id}` immediately after a successful DB update.

## Background worker flow

1. Payment publishes `payment.completed` after an authorized payment.
2. Notification consumes the queue with manual ack and QoS 1.
3. If `notification:sent:{order_id}` exists, the message is acked without sending again.
4. Otherwise the worker calls the configured `EmailSender` with exponential backoff retries.
5. After success the worker sets the Redis idempotency key and acks the message.
