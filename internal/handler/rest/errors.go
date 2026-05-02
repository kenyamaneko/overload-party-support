// Package rest は内部 REST API (:9009) の delivery 層。
package rest

import (
	"errors"
	"net/http"

	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
)

// errorStatus は service 層のエラーを HTTP ステータスコードに変換する。
func errorStatus(err error) int {
	switch {
	case errors.Is(err, announcement.ErrNotFound),
		errors.Is(err, inquiry.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, announcement.ErrLangRequired),
		errors.Is(err, announcement.ErrUnsupportedLang),
		errors.Is(err, inquiry.ErrInvalidStatusValue):
		return http.StatusBadRequest
	case errors.Is(err, inquiry.ErrInvalidStatusTransition):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
