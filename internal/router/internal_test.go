package router_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/router"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
)

func TestNewInternal(t *testing.T) {
	t.Run("[内部APIルータ]ヘルスチェック", func(t *testing.T) {
		t.Run("healthエンドポイントを呼び出すと、ステータス200と本文{\"status\":\"ok\"}を返す", func(t *testing.T) {
			handler := rest.NewAnnouncementHandler(announcement.New(&port.MockAnnouncementRepo{}, time.Now))
			r := router.NewInternal(handler)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/health", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 200, w.Code)
			assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
		})
	})
}
