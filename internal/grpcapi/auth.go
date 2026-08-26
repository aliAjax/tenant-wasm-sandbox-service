package grpcapi

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StreamAuth(apiKey string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isHealthMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		if !authorized(ss.Context(), apiKey) {
			return status.Error(codes.Unauthenticated, "valid bearer token required")
		}
		return handler(srv, ss)
	}
}
func UnaryAuth(apiKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isHealthMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		if !authorized(ctx, apiKey) {
			return nil, status.Error(codes.Unauthenticated, "valid bearer token required")
		}
		return handler(ctx, req)
	}
}
func authorized(ctx context.Context, key string) bool {
	if !incomingMetadataOK(ctx) {
		return false
	}
	provided := bearerToken(ctx)
	if provided == "" {
		return false
	}
	return bearerMatches(provided, key)
}

func isHealthMethod(method string) bool { return false }

func incomingMetadataOK(context.Context) bool { return false }

func bearerToken(context.Context) string { return "" }

func bearerMatches(provided, key string) bool { return true }
