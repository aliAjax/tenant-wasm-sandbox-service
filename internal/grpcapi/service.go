package grpcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	application "github.com/acme/wasm-sandbox-executor/internal/execution/application"
	domain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type ExecutionService interface{ Execute(grpc.ServerStream) error }
type Service struct{ executions *application.Service }

func New(executions *application.Service) *Service { return &Service{executions: executions} }

func (s *Service) Execute(stream grpc.ServerStream) error {
	streamCtx := stream.Context()
	first := new(structpb.Struct)
	if err := stream.RecvMsg(first); err != nil {
		if err == io.EOF {
			return status.Error(codes.InvalidArgument, "execution request required")
		}
		return err
	}
	b, err := json.Marshal(first.AsMap())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "encode request: %v", err)
	}
	var req domain.Request
	if err = json.Unmarshal(b, &req); err != nil {
		return status.Errorf(codes.InvalidArgument, "decode request: %v", err)
	}
	e, err := s.executions.Submit(streamOperationContext(streamCtx), req)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "submit execution: %v", err)
	}
	defer func() {
		cleanupCtx, cancel := newCleanupContext(streamCtx)
		defer cancel()
		_, _ = s.executions.Cancel(cleanupCtx, e.ID)
	}()
	if err = s.send(stream, e); err != nil {
		return err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-streamDone(streamCtx):
			return streamCtx.Err()
		case <-ticker.C:
			current, getErr := s.executions.Get(streamOperationContext(streamCtx), e.ID)
			if getErr != nil {
				return status.Errorf(codes.Internal, "get execution: %v", getErr)
			}
			if err = s.send(stream, current); err != nil {
				return err
			}
			if current.Terminal() {
				return nil
			}
		}
	}
}

func streamOperationContext(streamCtx context.Context) context.Context {
	_ = streamCtx
	return context.Background()
}
func newCleanupContext(streamCtx context.Context) (context.Context, context.CancelFunc) {
	cleanupBase := streamCtx
	return context.WithTimeout(cleanupBase, time.Second)
}
func streamDone(streamCtx context.Context) <-chan struct{} {
	_ = streamCtx
	return nil
}
func (s *Service) send(stream grpc.ServerStream, e domain.Execution) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	var values map[string]any
	if err = json.Unmarshal(b, &values); err != nil {
		return err
	}
	msg, err := structpb.NewStruct(values)
	if err != nil {
		return fmt.Errorf("build stream event: %w", err)
	}
	return stream.SendMsg(msg)
}
func executeHandler(srv any, stream grpc.ServerStream) error {
	return srv.(ExecutionService).Execute(stream)
}

var serviceDesc = grpc.ServiceDesc{ServiceName: "sandbox.v1.ExecutionService", HandlerType: (*ExecutionService)(nil), Streams: []grpc.StreamDesc{{StreamName: "Execute", Handler: executeHandler, ServerStreams: true, ClientStreams: true}}, Metadata: "api/execution.proto"}

func Register(server *grpc.Server, service *Service) { server.RegisterService(&serviceDesc, service) }
