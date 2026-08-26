package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	artifact "github.com/acme/wasm-sandbox-executor/internal/artifact/infrastructure"
	cacheinfra "github.com/acme/wasm-sandbox-executor/internal/cache/infrastructure"
	callback "github.com/acme/wasm-sandbox-executor/internal/callback/infrastructure"
	"github.com/acme/wasm-sandbox-executor/internal/config"
	execapp "github.com/acme/wasm-sandbox-executor/internal/execution/application"
	execdomain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	execinfra "github.com/acme/wasm-sandbox-executor/internal/execution/infrastructure"
	grpcapi "github.com/acme/wasm-sandbox-executor/internal/grpcapi"
	httpapi "github.com/acme/wasm-sandbox-executor/internal/http"
	moduleapp "github.com/acme/wasm-sandbox-executor/internal/module/application"
	queueapp "github.com/acme/wasm-sandbox-executor/internal/queue/application"
	queueinfra "github.com/acme/wasm-sandbox-executor/internal/queue/infrastructure"
	resource "github.com/acme/wasm-sandbox-executor/internal/resource/application"
	runtimeinfra "github.com/acme/wasm-sandbox-executor/internal/runtime/infrastructure"
	"github.com/acme/wasm-sandbox-executor/internal/store"
	worker "github.com/acme/wasm-sandbox-executor/internal/worker/application"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "configuration file")
	flag.Parse()
	if err := run(*configPath); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	state, err := store.New(cfg.Storage.StateFile)
	if err != nil {
		return err
	}
	recovered, err := state.RecoverInterrupted(context.Background())
	if err != nil {
		return fmt.Errorf("recover interrupted executions: %w", err)
	}
	if recovered > 0 {
		logger.Warn("recovered interrupted executions", "count", recovered)
	}
	ids := &execinfra.RandomIDs{}
	verifier := artifact.HMACVerifier{Secrets: map[string][]byte{}, AllowUnsigned: cfg.Security.AllowUnsignedModules}
	clock := moduleapp.SystemClock{}
	modules := moduleapp.New(state, verifier, clock, ids, cfg.Runtime.MaxModuleBytes)
	queue := queueinfra.NewFairQueue()
	scheduler := queueapp.NewScheduler(queue)
	cache := cacheinfra.NewMemory()
	resources := resource.NewManager(cfg.Tenant.MaxConcurrent, cfg.Tenant.MaxCPUPerMinute, cfg.Tenant.MaxMemoryPages)
	rt := runtimeinfra.NewSimulator(cfg.Runtime.Version)
	meter := execinfra.NewMemoryMeter()
	webhook := callback.NewWebhook(time.Second, 3)
	execRepo := store.ExecutionRepository{Store: state}
	maxBudget := execdomain.Budget{MaxInstructions: cfg.Runtime.MaxInstructions, MaxMemoryPages: cfg.Runtime.MaxMemoryPages, MaxStackBytes: cfg.Runtime.MaxStackBytes, Timeout: cfg.Runtime.MaxExecutionTime, MaxOutputBytes: cfg.Runtime.MaxOutputBytes}
	executions := execapp.New(execRepo, state, rt, cache, resources, scheduler, meter, webhook, ids, clock, execapp.Options{MaxBudget: maxBudget, NodeID: cfg.Node.ID, CacheTTL: cfg.Runtime.CacheTTL})
	nodes := worker.NewNodeRegistry(cfg.Node.ID, cfg.Worker.Count)
	metrics := httpapi.NewMetrics(queue.Len, resources.Active)
	router := httpapi.NewRouter(httpapi.Dependencies{Modules: modules, Executions: executions, Resources: resources, Cache: cache, Nodes: nodes, Runtime: rt, Metrics: metrics, MaxModuleBytes: cfg.Runtime.MaxModuleBytes})
	middleware := httpapi.NewMiddleware(logger, cfg.Security.APIKey, cfg.HTTP.RequestTimeout, cfg.HTTP.MaxBodyBytes, cfg.HTTP.RatePerSecond, metrics)
	httpServer := &http.Server{Addr: cfg.HTTP.Address, Handler: middleware.Wrap(router.Handler()), ReadTimeout: cfg.HTTP.ReadTimeout, WriteTimeout: cfg.HTTP.WriteTimeout, IdleTimeout: 30 * time.Second}
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(grpcapi.StreamAuth(cfg.Security.APIKey)), grpc.UnaryInterceptor(grpcapi.UnaryAuth(cfg.Security.APIKey)))
	grpcapi.Register(grpcServer, grpcapi.New(executions))
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	httpListener, err := net.Listen("tcp", cfg.HTTP.Address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	grpcListener, err := net.Listen("tcp", cfg.GRPC.Address)
	if err != nil {
		_ = httpListener.Close()
		return fmt.Errorf("listen gRPC: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool := worker.NewPool(queue, scheduler, executions, logger, cfg.Worker.Count)
	pool.Start(ctx)
	errCh := make(chan error, 2)
	go func() {
		logger.Info("HTTP server listening", "address", httpListener.Addr().String())
		if serveErr := httpServer.Serve(httpListener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()
	go func() {
		logger.Info("gRPC server listening", "address", grpcListener.Addr().String())
		if serveErr := grpcServer.Serve(grpcListener); serveErr != nil {
			errCh <- serveErr
		}
	}()
	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case serveErr := <-errCh:
		stop()
		logger.Error("listener failed", "error", serveErr)
	}
	nodes.Stop()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = queue.Close()
	if err = httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}
	grpcDone := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcDone) }()
	select {
	case <-grpcDone:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	pool.Wait()
	_ = rt.Close(shutdownCtx)
	return nil
}
