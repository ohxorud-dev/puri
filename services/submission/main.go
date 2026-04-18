package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/puri-cp/puri/services/submission/config"
	"github.com/puri-cp/puri/services/submission/db"
	"github.com/puri-cp/puri/services/submission/executor"
	"github.com/puri-cp/puri/services/submission/handler"
	"github.com/puri-cp/puri/services/submission/interceptor"
	"github.com/puri-cp/puri/services/submission/queue"
	"github.com/puri-cp/puri/services/submission/repository"

	submissionv1connect "github.com/puri-cp/puri/gen/submission/v1/submissionv1connect"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()


	runner, err := executor.NewDockerRunner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create docker runner: %v\n", err)
		os.Exit(1)
	}

	publisher, err := queue.NewPublisher(cfg.RabbitMQURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to rabbitmq publisher: %v\n", err)
		os.Exit(1)
	}
	defer publisher.Close()

	subRepo := repository.NewSubmissionRepository(pool)

	consumer, err := queue.NewConsumer(cfg.RabbitMQURL, subRepo, runner, cfg.ProblemsPath, cfg.TestcasesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to rabbitmq consumer: %v\n", err)
		os.Exit(1)
	}
	consumer.Start()

	orphanCtx, orphanCancel := context.WithCancel(context.Background())
	defer orphanCancel()
	go runOrphanResetLoop(orphanCtx, subRepo)

	valInterceptor, err := interceptor.NewValidationInterceptor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create validation interceptor: %v\n", err)
		os.Exit(1)
	}

	logInterceptor := interceptor.NewLoggingInterceptor()
	opts := []connect.HandlerOption{
		connect.WithInterceptors(valInterceptor, logInterceptor),
	}

	subHandler := handler.NewSubmissionServiceHandler(subRepo, runner, publisher, cfg.ProblemsPath)

	mux := http.NewServeMux()
	submissionPath, submissionRoute := submissionv1connect.NewSubmissionServiceHandler(subHandler, opts...)
	mux.Handle(submissionPath, submissionRoute)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    ":" + cfg.SubmissionPort,
		Handler: mux,
	}

	go func() {
		fmt.Printf("Submission server starting on :%s\n", cfg.SubmissionPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down...")

	orphanCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "server shutdown error: %v\n", err)
	}
	if err := consumer.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "consumer shutdown error: %v\n", err)
	}
	fmt.Println("Server stopped gracefully")
}

func runOrphanResetLoop(ctx context.Context, repo *repository.SubmissionRepository) {
	const maxJudgingAge = 3 * time.Minute
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	resetOnce := func() {
		opCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		n, err := repo.ResetOrphanedJudging(opCtx, maxJudgingAge)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[orphan-reset] failed: %v\n", err)
			return
		}
		if n > 0 {
			fmt.Printf("[orphan-reset] reset %d stuck JUDGING submissions\n", n)
		}
	}

	resetOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resetOnce()
		}
	}
}
