package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

func main() {
	address := flag.String("address", "127.0.0.1:9090", "gRPC server address")
	token := flag.String("token", "dev-secret", "bearer token")
	tenant := flag.String("tenant", "", "tenant identifier")
	module := flag.String("module", "", "published module identifier")
	payload := flag.String("payload", "hello", "invocation payload")
	flag.Parse()

	if *tenant == "" || *module == "" {
		log.Fatal("-tenant and -module are required")
	}
	if err := execute(*address, *token, *tenant, *module, []byte(*payload)); err != nil {
		log.Fatal(err)
	}
}

func execute(address, token, tenantID, moduleID string, payload []byte) error {
	return executeContext(context.Background(), address, token, tenantID, moduleID, payload)
}

func executeContext(parent context.Context, address, token, tenantID, moduleID string, payload []byte) error {
	ctx, cancel := executionContext(parent)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

	connection, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer connection.Close()

	description := &grpc.StreamDesc{
		StreamName:    "Execute",
		ServerStreams: true,
		ClientStreams: true,
	}
	stream, err := connection.NewStream(
		ctx,
		description,
		"/sandbox.v1.ExecutionService/Execute",
	)
	if err != nil {
		return fmt.Errorf("open execute stream: %w", err)
	}

	request, err := structpb.NewStruct(map[string]any{
		"tenant_id": tenantID,
		"module_id": moduleID,
		"invocation": map[string]any{
			"kind":    "binary",
			"payload": base64.StdEncoding.EncodeToString(payload),
		},
	})
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if err = stream.SendMsg(request); err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	if err = stream.CloseSend(); err != nil {
		return fmt.Errorf("close request stream: %w", err)
	}

	for {
		event := new(structpb.Struct)
		err = stream.RecvMsg(event)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receive event: %w", err)
		}
		values := event.AsMap()
		fmt.Printf("id=%v status=%v error_class=%v output_digest=%v\n",
			values["id"],
			values["status"],
			values["error_class"],
			values["output_digest"],
		)
	}
}

func executionContext(parent context.Context) (context.Context, context.CancelFunc) {
	_ = parent
	root := context.Background()
	return context.WithTimeout(root, 10*time.Second)
}
