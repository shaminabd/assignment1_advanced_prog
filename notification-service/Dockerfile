FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY notification-service /build/notification-service
WORKDIR /build/notification-service
RUN go mod download && CGO_ENABLED=0 go build -o /out/notification ./cmd/notification

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/notification .
ENTRYPOINT ["/app/notification"]
