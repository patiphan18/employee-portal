// Command server runs the Employee Portal API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/digithun/employee-portal/backend/internal/api"
	"github.com/digithun/employee-portal/backend/internal/auth"
	"github.com/digithun/employee-portal/backend/internal/config"
	"github.com/digithun/employee-portal/backend/internal/service"
	"github.com/digithun/employee-portal/backend/internal/storage"
	"github.com/digithun/employee-portal/backend/internal/store"

	_ "github.com/lib/pq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL, 60*time.Second)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	slog.Info("migrations applied")

	st := store.NewPostgres(db)

	if cfg.SeedDemoUsers {
		if err := seedDemoUsers(ctx, st); err != nil {
			return err
		}
	} else {
		slog.Info("demo users not seeded", "env", cfg.Env)
	}

	files, err := storage.NewLocal(cfg.UploadDir)
	if err != nil {
		return err
	}

	profileSvc := service.NewProfile(st)
	srv := &api.Server{
		Store:          st,
		Issuer:         auth.NewIssuer(cfg.JWTSecret, time.Duration(cfg.JWTTTLMinutes)*time.Minute),
		Profile:        profileSvc,
		History:        service.NewHistory(st, cfg.HistoryRetentionDays),
		Admin:          service.NewAdmin(st, cfg.HistoryRetentionDays),
		Storage:        files,
		MaxUploadBytes: cfg.MaxUploadBytes,

		TrustProxyHeaders: cfg.TrustProxyHeaders,
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Routes(cfg.CORSOrigins, cfg.UploadDir),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "port", cfg.Port, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// seedDemoUsers creates the two accounts from the acceptance criteria if they are
// absent. Hashing happens here rather than in SQL so the seed uses the same bcrypt
// settings as login.
//
// These passwords are published in the README, so this must never run in
// production. config.Load forces SeedDemoUsers off when APP_ENV=production, and
// the caller honours that flag.
func seedDemoUsers(ctx context.Context, st *store.Postgres) error {
	demo := []struct {
		Email, Password, First, Last, Role, Mobile string
	}{
		{"somchai@example.com", "Password123!", "Somchai", "Jaidee", "employee", "0812345678"},
		{"admin@example.com", "Admin123!", "Araya", "Suksan", "admin", "0898765432"},
	}

	for _, d := range demo {
		if _, err := st.UserByEmail(ctx, d.Email); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		hash, err := auth.HashPassword(d.Password)
		if err != nil {
			return err
		}
		if err := st.InsertUser(ctx, d.Email, hash, d.First, d.Last, d.Role, d.Mobile); err != nil {
			return err
		}
		slog.Info("seeded demo user", "email", d.Email)
	}
	return nil
}
