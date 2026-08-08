// Command server は support サービスの実行バイナリ。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kenyamaneko/overload-party-support/internal/config"
	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-support/internal/router"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
)

func main() {
	if err := run(); err != nil {
		slog.Error("support fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	if err := setupLogger(cfg.Env); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, closeDatabasePool, err := newDatabasePool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("new database pool: %w", err)
	}
	defer closeDatabasePool()
	defer pool.Close()

	announcementRepo := postgres.NewAnnouncementRepository(pool)

	announcementUsecase := announcement.New(announcementRepo, time.Now)
	announcementH := rest.NewAnnouncementHandler(announcementUsecase)

	internalSrv := &http.Server{
		Handler:           router.NewInternal(announcementH),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.InternalPort))
	if err != nil {
		return fmt.Errorf("listen internal port: %w", err)
	}

	slog.Info("listening",
		"internal_addr", ln.Addr().String(),
		"env", cfg.Env,
	)

	return serve(ctx, internalSrv, ln)
}

// setupLogger は env に応じて slog のハンドラを設定する。
func setupLogger(env config.Env) error {
	switch env {
	case config.EnvProduction, config.EnvStaging:
		slog.SetDefault(slog.New(newCloudLoggingHandler()).With("service", "support"))
	case config.EnvLocal:
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(h).With("service", "support"))
	default:
		return fmt.Errorf("unexpected ENV: %s", env)
	}
	return nil
}

// newCloudLoggingHandler は Cloud Logging が認識するフィールド名に slog 属性をリネームする。
func newCloudLoggingHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				a.Key = "severity"
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					switch {
					case lvl >= slog.LevelError:
						a.Value = slog.StringValue("ERROR")
					case lvl >= slog.LevelWarn:
						a.Value = slog.StringValue("WARNING")
					case lvl >= slog.LevelInfo:
						a.Value = slog.StringValue("INFO")
					default:
						a.Value = slog.StringValue("DEBUG")
					}
				}
			}
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	})
}

// serve は ln で HTTP server を起動し、シグナルまたは ctx の終了で graceful shutdown する。
func serve(ctx context.Context, srv *http.Server, ln net.Listener) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("internal http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("internal http shutdown: %w", err)
		}
		return nil
	})

	return g.Wait()
}
