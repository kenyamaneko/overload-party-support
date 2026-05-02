// Package admin は運用者向け管理 UI (HTMX + html/template) の delivery 層。
package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/config"
)

// iapEmailHeader は IAP が認証成功時に付与するヘッダ名。
// 値は "<identity-provider>:<email>" 形式 (例: "accounts.google.com:user@example.com")。
const iapEmailHeader = "X-Goog-Authenticated-User-Email"

// localReviewerFallback は ENV=local のときに reviewer として注入する固定値。
const localReviewerFallback = "local-dev@example.com"

// ErrMissingIAPHeader は IAP ヘッダが欠けているときに返す。
var ErrMissingIAPHeader = errors.New("missing IAP authenticated user header")

// AuthMiddleware は IAP ヘッダの存在確認と reviewer の context 注入を行う gin middleware を返す。
func AuthMiddleware(env config.Env) gin.HandlerFunc {
	if env == config.EnvLocal {
		return func(c *gin.Context) {
			c.Set(reviewerKey(), localReviewerFallback)
			c.Next()
		}
	}
	return func(c *gin.Context) {
		email, err := extractIAPEmail(c.GetHeader(iapEmailHeader))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set(reviewerKey(), email)
		c.Next()
	}
}

// reviewerKey は gin.Context.Set/Get に使う文字列キー。
func reviewerKey() string {
	return "admin.reviewer"
}

// Reviewer は IAP 由来の運用者 email を取り出す。middleware 未適用時は空文字列。
func Reviewer(c *gin.Context) string {
	v, ok := c.Get(reviewerKey())
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// extractIAPEmail は IAP ヘッダ値から email 部分を取り出す。
// 想定フォーマット: "<provider>:<email>" (プレフィックス無しも許容)。
func extractIAPEmail(raw string) (string, error) {
	if raw == "" {
		return "", ErrMissingIAPHeader
	}
	if idx := strings.Index(raw, ":"); idx >= 0 {
		email := raw[idx+1:]
		if email == "" {
			return "", ErrMissingIAPHeader
		}
		return email, nil
	}
	return raw, nil
}
