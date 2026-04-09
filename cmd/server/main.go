// cmd/server/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"RepoWatch/db"
	"RepoWatch/internal/config"
	"RepoWatch/internal/email"
	"RepoWatch/internal/release"
	"RepoWatch/internal/subscription"
)

// repoAdapter bridges subscription.PostgresRepository to release.SubscriptionSource.
// Defined here to avoid a circular import between release and subscription packages.
type repoAdapter struct {
	r *subscription.PostgresRepository
}

func (a *repoAdapter) FindAllConfirmed(ctx context.Context) ([]release.SubscriptionEntry, error) {
	subs, err := a.r.FindAllConfirmed(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]release.SubscriptionEntry, len(subs))
	for i, s := range subs {
		entries[i] = release.SubscriptionEntry{
			ID:          s.ID,
			Email:       s.Email,
			Repo:        s.Repo,
			UnsubToken:  s.UnsubToken,
			LastSeenTag: s.LastSeenTag,
		}
	}
	return entries, nil
}

func (a *repoAdapter) UpdateLastSeenTag(ctx context.Context, id, tag string) error {
	return a.r.UpdateLastSeenTag(ctx, id, tag)
}

func main() {
	cfg := config.Load()

	// Connect to PostgreSQL and run migrations before starting anything else.
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	// Build components.
	notifier := email.NewSMTPNotifier(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	githubClient := release.NewGitHubClient(cfg.GitHubToken, "")
	repo := subscription.NewPostgresRepository(pool)
	svc := subscription.NewService(repo, githubClient, notifier, cfg.Host)
	handler := subscription.NewHandler(svc)
	scanner := release.NewScanner(&repoAdapter{repo}, githubClient, notifier, cfg.Host)

	// Register routes.
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Post("/api/subscribe", handler.Subscribe)
	r.Get("/api/confirm/{token}", handler.Confirm)
	r.Get("/api/unsubscribe/{token}", handler.Unsubscribe)
	r.Get("/api/subscriptions", handler.ListSubscriptions)

	// Start the scanner in the background.
	scanCtx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()
	go scanner.Start(scanCtx, cfg.ScanInterval)

	// Start HTTP server.
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	// Wait for SIGINT or SIGTERM, then gracefully shut down.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
