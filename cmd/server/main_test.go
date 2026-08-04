package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/router"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
)

func TestServe(t *testing.T) {
	t.Run("gateway 向け内部 API サーバの起動と停止", func(t *testing.T) {
		t.Run("起動中はお知らせ一覧の取得が 200 を返し、停止要求で終了する", func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			srv := &http.Server{
				Handler:           newInternalRouterWithStubs(),
				ReadHeaderTimeout: 10 * time.Second,
			}
			ctx, cancel := context.WithCancel(context.Background())
			served := make(chan error, 1)
			go func() { served <- serve(ctx, srv, ln) }()

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get("http://" + ln.Addr().String() + "/api/v1/support/announcements?lang=ja")
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			cancel()
			require.NoError(t, <-served)
		})
	})
}

// newInternalRouterWithStubs はお知らせなしを返す repo で内部 API のルータを構築する。
func newInternalRouterWithStubs() http.Handler {
	querier := &port.MockAnnouncementRepo{
		ListPublishedFn: func(context.Context, string, time.Time) ([]domain.AnnouncementSummary, error) {
			return []domain.AnnouncementSummary{}, nil
		},
	}
	return router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))
}
