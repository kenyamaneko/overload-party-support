// Command server は support サービスの実行バイナリ。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kenyamaneko/overload-party-support/internal/adapter/sendgrid"
	"github.com/kenyamaneko/overload-party-support/internal/adapter/sendgrid/sendgridnoop"
	"github.com/kenyamaneko/overload-party-support/internal/adapter/slack"
	"github.com/kenyamaneko/overload-party-support/internal/adapter/slack/slacknoop"
	"github.com/kenyamaneko/overload-party-support/internal/config"
	"github.com/kenyamaneko/overload-party-support/internal/handler/admin"
	"github.com/kenyamaneko/overload-party-support/internal/handler/external"
	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-support/internal/router"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/usecase/announcement_admin"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
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
	inquiryRepo := postgres.NewInquiryRepository(pool)

	slackNotifier := pickSlackNotifier(cfg)
	emailSender, err := pickEmailSender(cfg)
	if err != nil {
		return fmt.Errorf("build email sender: %w", err)
	}

	announcementUsecase := announcement.New(announcementRepo, time.Now)
	announcementAdminUsecase := announcementadmin.New(announcementRepo, time.Now)
	inquiryUsecase := inquiry.New(inquiryRepo, slackNotifier, emailSender, cfg.InquiryBodySnippetLength)

	announcementH := rest.NewAnnouncementHandler(announcementUsecase)
	externalH := external.NewInquiryHandler(inquiryUsecase)
	adminH, err := admin.NewHandler(announcementAdminUsecase, time.Now)
	if err != nil {
		return fmt.Errorf("build admin handler: %w", err)
	}

	internalSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.InternalPort),
		Handler:           router.NewInternal(announcementH),
		ReadHeaderTimeout: 10 * time.Second,
	}
	adminSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.AdminPort),
		Handler:           router.NewAdmin(cfg.Env, adminH),
		ReadHeaderTimeout: 10 * time.Second,
	}
	externalSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ExternalPort),
		Handler:           router.NewExternal(cfg.CORSAllowedOrigins, externalH),
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("listening",
		"internal_addr", internalSrv.Addr,
		"admin_addr", adminSrv.Addr,
		"external_addr", externalSrv.Addr,
		"env", cfg.Env,
	)

	return runAll(ctx, internalSrv, adminSrv, externalSrv)
}

// pickSlackNotifier は ENV に応じて real / noop を選択する。
func pickSlackNotifier(cfg *config.Config) port.SlackNotifier {
	if cfg.Env == config.EnvLocal {
		return slacknoop.New()
	}
	return slack.NewRealNotifier(cfg.SlackBotToken, cfg.SlackChannelID)
}

// pickEmailSender は ENV に応じて real / noop を選択する。
func pickEmailSender(cfg *config.Config) (port.EmailSender, error) {
	if cfg.Env == config.EnvLocal {
		return sendgridnoop.New(), nil
	}
	return sendgrid.NewRealSender(cfg.SendGridAPIKey, cfg.SendGridFromAddress, cfg.SendGridFromName)
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

// runAll は 3 つの HTTP server を並行起動し、いずれかの失敗・シグナルで全員を停止させる。
func runAll(ctx context.Context, internalSrv, adminSrv, externalSrv *http.Server) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := internalSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("internal http server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("admin http server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := externalSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("external http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := internalSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("internal http shutdown: %w", err)
		}
		if err := adminSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("admin http shutdown: %w", err)
		}
		if err := externalSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("external http shutdown: %w", err)
		}
		return nil
	})

	return g.Wait()
}
