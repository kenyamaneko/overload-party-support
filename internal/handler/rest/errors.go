package rest

import (
	"errors"
	"net/http"

	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
)

// errorStatus は service 層のエラーを HTTP ステータスコードに変換する。
func errorStatus(err error) int {
	switch {
	case errors.Is(err, announcement.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, announcement.ErrLangRequired),
		errors.Is(err, announcement.ErrUnsupportedLang):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
