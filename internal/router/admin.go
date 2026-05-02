package router

import (
	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/config"
	"github.com/kenyamaneko/overload-party-support/internal/handler/admin"
)

// NewAdmin は管理 UI (:9109) のルータを構築する。
func NewAdmin(env config.Env, adminH *admin.Handler) *gin.Engine {
	r := gin.New()
	r.Use(requestLogger(), gin.Recovery())

	r.GET("/health", healthHandler)

	adminGroup := r.Group("/admin", admin.AuthMiddleware(env))
	{
		adminGroup.GET("/announcements", adminH.List)
		adminGroup.GET("/announcements/new", adminH.ShowNew)
		adminGroup.POST("/announcements", adminH.Create)
		adminGroup.GET("/announcements/:announcementId", adminH.ShowEdit)
		adminGroup.POST("/announcements/:announcementId", adminH.Update)
		adminGroup.POST("/announcements/:announcementId/delete", adminH.Delete)
		adminGroup.POST("/announcements/:announcementId/translations/:lang", adminH.UpsertTranslation)
	}
	return r
}
