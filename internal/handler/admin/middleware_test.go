package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-support/internal/config"
	"github.com/kenyamaneko/overload-party-support/internal/handler/admin"
)

// 仕様 (FEATURE_SPEC §6.2 / ARCHITECTURE): production/staging では IAP ヘッダ必須。
// ヘッダ値の email 部分を reviewer として context に注入する。
func TestAuthMiddleware_RequireIAPInProduction(t *testing.T) {
	cases := []struct {
		name         string
		header       string
		wantStatus   int
		wantReviewer string
	}{
		{
			name:         "プロバイダープレフィックス付き",
			header:       "accounts.google.com:alice@example.com",
			wantStatus:   http.StatusOK,
			wantReviewer: "alice@example.com",
		},
		{
			name:         "プレフィックスなしも許容",
			header:       "bob@example.com",
			wantStatus:   http.StatusOK,
			wantReviewer: "bob@example.com",
		},
		{
			name:         "ヘッダ不在は 401",
			header:       "",
			wantStatus:   http.StatusUnauthorized,
			wantReviewer: "",
		},
		{
			name:         "プレフィックスのみで email 空は 401",
			header:       "accounts.google.com:",
			wantStatus:   http.StatusUnauthorized,
			wantReviewer: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			var seenReviewer string
			r := gin.New()
			r.Use(admin.AuthMiddleware(config.EnvProduction))
			r.GET("/_probe", func(c *gin.Context) {
				seenReviewer = admin.Reviewer(c)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/_probe", nil)
			switch tc.header {
			case "":
				// no header
			default:
				req.Header.Set("X-Goog-Authenticated-User-Email", tc.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantReviewer, seenReviewer)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.2): local 環境ではヘッダ不要で固定 reviewer を注入する。
func TestAuthMiddleware_LocalSkipsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var seenReviewer string
	r := gin.New()
	r.Use(admin.AuthMiddleware(config.EnvLocal))
	r.GET("/_probe", func(c *gin.Context) {
		seenReviewer = admin.Reviewer(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/_probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, seenReviewer, "local はヘッダ無しでも reviewer が注入されるべき")
}
