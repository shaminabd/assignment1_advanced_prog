package grpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

func UnaryLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("gRPC %s duration=%s", info.FullMethod, time.Since(start))
	return resp, err
}
