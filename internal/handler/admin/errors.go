package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/service/announcement_admin"
)

// errorStatus はサービス層のエラーを HTTP ステータスにマップする。
func errorStatus(err error) int {
	switch {
	case errors.Is(err, announcementadmin.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, announcementadmin.ErrInvalidType),
		errors.Is(err, announcementadmin.ErrUnsupportedLang),
		errors.Is(err, announcementadmin.ErrInvalidField),
		errors.Is(err, announcementadmin.ErrInvalidStatusFilter):
		return http.StatusBadRequest
	case errors.Is(err, ErrMissingIAPHeader):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// respondError は管理 UI のエラーをテキストで返す。HTMX は 4xx/5xx で hx-swap を抑止する。
func respondError(c *gin.Context, err error) {
	c.String(errorStatus(err), "%s", err.Error())
}
