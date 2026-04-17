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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/cors"
	"github.com/puri-cp/puri/services/api/config"
	"github.com/puri-cp/puri/services/api/db"
	"github.com/puri-cp/puri/services/api/handler"
	"github.com/puri-cp/puri/services/api/interceptor"
	"github.com/puri-cp/puri/services/api/repository"

	communityv1connect "github.com/puri-cp/puri/gen/community/v1/communityv1connect"
	proposalv1connect "github.com/puri-cp/puri/gen/proposal/v1/proposalv1connect"
	submissionv1connect "github.com/puri-cp/puri/gen/submission/v1/submissionv1connect"
	userv1connect "github.com/puri-cp/puri/gen/user/v1/userv1connect"
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

	if err := runMigrations(pool); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	valInterceptor, err := interceptor.NewValidationInterceptor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create validation interceptor: %v\n", err)
		os.Exit(1)
	}

	logInterceptor := interceptor.NewLoggingInterceptor()
	authInterceptor := interceptor.NewAuthInterceptor(cfg.JWTSecret)

	opts := []connect.HandlerOption{
		connect.WithInterceptors(valInterceptor, logInterceptor, authInterceptor),
	}

	userRepo := repository.NewUserRepository(pool)
	communityRepo := repository.NewCommunityRepository(pool)
	userHandler := handler.NewUserServiceHandler(userRepo, cfg.JWTSecret, cfg.Env == "production")
	communityHandler := handler.NewCommunityServiceHandler(communityRepo, userRepo)
	submissionHandler := handler.NewSubmissionServiceHandler(cfg.SubmissionServiceURL)
	proposalRepo := repository.NewProposalRepository(pool)
	proposalHandler := handler.NewProposalServiceHandler(proposalRepo, cfg.SubmissionServiceURL)

	mux := http.NewServeMux()

	userPath, userRoute := userv1connect.NewUserServiceHandler(userHandler, opts...)
	mux.Handle(userPath, userRoute)

	communityPath, communityRoute := communityv1connect.NewCommunityServiceHandler(communityHandler, opts...)
	mux.Handle(communityPath, communityRoute)

	submissionPath, submissionRoute := submissionv1connect.NewSubmissionServiceHandler(submissionHandler, opts...)
	mux.Handle(submissionPath, submissionRoute)

	proposalPath, proposalRoute := proposalv1connect.NewProposalServiceHandler(proposalHandler, opts...)
	mux.Handle(proposalPath, proposalRoute)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://puri.ac", "http://localhost:4321"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Connect-Protocol-Version"},
		AllowCredentials: true,
	}).Handler(mux)

	server := &http.Server{
		Addr:    ":" + cfg.APIPort,
		Handler: corsHandler,
	}

	go func() {
		fmt.Printf("API server starting on :%s\n", cfg.APIPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "server shutdown error: %v\n", err)
	}
	fmt.Println("Server stopped gracefully")
}

func runMigrations(pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose: failed to set dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "../../db/migrations"); err != nil {
		return fmt.Errorf("goose: failed to run migrations: %w", err)
	}

	return nil
}
