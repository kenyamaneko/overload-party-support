package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/handler/external"
)

// NewExternal は問い合わせフォーム (:9209) 向けの公開ルータを構築する。
func NewExternal(allowedOrigins []string, inq *external.InquiryHandler) *gin.Engine {
	r := gin.New()
	r.Use(newRequestLogger(), gin.Recovery())
	r.Use(newCORSMiddleware(allowedOrigins))

	r.GET("/health", healthHandler)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/inquiries", inq.Submit)
	}
	return r
}

// newCORSMiddleware は許可オリジンのみ Access-Control-Allow-Origin を返す。
func newCORSMiddleware(allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowedSet[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
