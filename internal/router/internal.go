// Package router は HTTP ルータ構築を一箇所に集める。
// 内部 REST (:9009) / 管理 UI (:9109) / 外部フォーム (:9209) は信頼境界が異なるため別ルータとして分離する (ARCHITECTURE.md)。
package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
)

// NewInternal は gateway / slack-commands 向け内部 REST API のルータを構築する。
// 認証は呼び出し元 (gateway や slack-commands) で完了している前提で、support 側では追加認証を行わない。
func NewInternal(ann *rest.AnnouncementHandler, inq *rest.InquiryHandler) *gin.Engine {
	r := gin.New()
	r.Use(requestLogger(), gin.Recovery())

	r.GET("/health", healthHandler)

	v1 := r.Group("/internal/v1")
	{
		v1.GET("/announcements", ann.List)
		v1.GET("/announcements/:announcementId", ann.GetDetail)

		v1.GET("/inquiries", inq.List)
		v1.GET("/inquiries/:inquiryId", inq.GetDetail)
		v1.POST("/inquiries/:inquiryId/status", inq.UpdateStatus)
		v1.POST("/inquiries/:inquiryId/note", inq.UpdateNote)
	}
	return r
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// requestLogger は構造化ログを出力する共通ミドルウェア。
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		slog.LogAttrs(c.Request.Context(), level, "http request",
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
		)
	}
}
