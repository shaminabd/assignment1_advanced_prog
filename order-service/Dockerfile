FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY rpcgen /build/rpcgen
COPY order-service /build/order-service
WORKDIR /build/order-service
RUN go mod download && CGO_ENABLED=0 go build -o /out/order ./cmd/order

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/order .
EXPOSE 8083 50051
ENTRYPOINT ["/app/order"]
