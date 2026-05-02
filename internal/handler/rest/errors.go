// Package rest は内部 REST API (:9009) の delivery 層。
// gateway / slack-commands からの REST 呼び出しを受け、service 層に委譲する。
package rest

import (
	"errors"
	"net/http"

	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
)

// errorStatus は service 層のエラーを HTTP ステータスコードに変換する。
// service 層は HTTP を知らず、変換は handler 層の責務 (FEATURE_SPEC §10)。
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
