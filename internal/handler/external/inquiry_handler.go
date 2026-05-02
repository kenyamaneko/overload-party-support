// Package external は外部公開フォーム (:9209) の delivery 層。
package external

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// InquiryHandler は問い合わせフォームから直接呼ばれる受付 API。
type InquiryHandler struct {
	uc *inquiry.Usecase
}

// NewInquiryHandler は InquiryHandler を生成する。
func NewInquiryHandler(uc *inquiry.Usecase) *InquiryHandler {
	return &InquiryHandler{uc: uc}
}

// Submit は `POST /api/v1/inquiries` を処理する。
func (h *InquiryHandler) Submit(c *gin.Context) {
	var req apisupport.SubmitInquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	id, err := h.uc.Submit(c.Request.Context(), req.Title, req.Body, req.ReplyEmail)
	if err != nil {
		slog.Warn("inquiry submit failed", "error", err)
		c.JSON(submitErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, apisupport.SubmitInquiryResponse{InquiryID: id})
}

// submitErrorStatus は Submit 特有のエラー分類。
func submitErrorStatus(err error) int {
	switch {
	case errors.Is(err, inquiry.ErrInvalidInquiry), errors.Is(err, inquiry.ErrInvalidEmail):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
