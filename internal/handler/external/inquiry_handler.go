// Package external は外部公開フォーム (:9209) の delivery 層。
// 認証なし、CORS で Origin を制限して問い合わせ受付のみを行う (FEATURE_SPEC §7.5)。
package external

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/service/inquiry"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// InquiryHandler は問い合わせフォームから直接呼ばれる受付 API。
type InquiryHandler struct {
	svc *inquiry.Service
}

// NewInquiryHandler は InquiryHandler を生成する。
func NewInquiryHandler(svc *inquiry.Service) *InquiryHandler {
	return &InquiryHandler{svc: svc}
}

// Submit は `POST /api/v1/inquiries` を処理する (FEATURE_SPEC §7.1)。
func (h *InquiryHandler) Submit(c *gin.Context) {
	var req apisupport.SubmitInquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	id, err := h.svc.Submit(c.Request.Context(), req.Title, req.Body, req.ReplyEmail)
	if err != nil {
		slog.Warn("inquiry submit failed", "error", err)
		c.JSON(submitErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, apisupport.SubmitInquiryResponse{InquiryID: id})
}

// submitErrorStatus は Submit 特有のエラー分類。
// バリデーション系は 400、副作用 (Slack / SendGrid / DB) 失敗はすべて 500。
func submitErrorStatus(err error) int {
	switch {
	case errors.Is(err, inquiry.ErrInvalidInquiry), errors.Is(err, inquiry.ErrInvalidEmail):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
