FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY rpcgen /build/rpcgen
COPY payment-service /build/payment-service
WORKDIR /build/payment-service
RUN go mod download && CGO_ENABLED=0 go build -o /out/payment ./cmd/payment

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/payment .
EXPOSE 8084 50052
ENTRYPOINT ["/app/payment"]
