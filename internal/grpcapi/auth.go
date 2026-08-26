package grpcapi

import (
	"context"
	"crypto/subtle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

type detachedServerStream struct{ grpc.ServerStream }

func (detachedServerStream) Context() context.Context { return context.Background() }

func StreamAuth(apiKey string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !authorized(ss.Context(), apiKey) {
			return status.Error(codes.Unauthenticated, "valid bearer token required")
		}
		return handler(srv, detachedServerStream{ServerStream: ss})
	}
}
func UnaryAuth(apiKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}
		if !authorized(ctx, apiKey) {
			return nil, status.Error(codes.Unauthenticated, "valid bearer token required")
		}
		return handler(ctx, req)
	}
}
func authorized(ctx context.Context, key string) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return false
	}
	provided := strings.TrimPrefix(values[0], "Bearer ")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(key)) == 1
}
