# Assignment 3 Architecture

## Architecture Diagram (Assignment 3)

```text
                          REST (Gin)                    gRPC ProcessPayment
                    ┌──────────────────┐      ┌────────────────────────────────────┐
  Client ──────────►│   Order Service  │─────►│         Payment Service            │
                    │    HTTP :8083    │      │   gRPC :50052 / HTTP :8084        │
                    └────────┬─────────┘      └─────────────────┬──────────────────┘
                             │                                  │
                             │ write order                      │ write payment
                             ▼                                  ▼
                       ┌────────────┐                     ┌─────────────┐
                       │  order_db  │                     │ payment_db  │
                       └────────────┘                     └─────────────┘
                                                               │
                                                               │ publish event (persistent + confirms)
                                                               ▼
                         ┌────────────────────────────────────────────────────────────┐
                         │ RabbitMQ                                                   │
                         │ exchange: payment.events (direct, durable)                 │
                         │ queue:    payment.completed (durable, manual ack consumer) │
                         └───────────────────────────────┬────────────────────────────┘
                                                         │ consume (autoAck=false)
                                                         ▼
                                              ┌──────────────────────────┐
                                              │ Notification Service     │
                                              │ - idempotency by order_id│
                                              │ - ack after stdout log   │
                                              └─────────────┬────────────┘
                                                            │
                                      ▼
                          ┌─────────────────────────┐
                          │ [Notification] log line │
                          │ email/order/amount      │
                          └─────────────────────────┘
```
