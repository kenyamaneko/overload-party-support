package rest

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/presenter"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// InquiryHandler は slack-commands が呼び出す問い合わせ管理 API。
type InquiryHandler struct {
	uc *inquiry.Usecase
}

// NewInquiryHandler は InquiryHandler を生成する。
func NewInquiryHandler(uc *inquiry.Usecase) *InquiryHandler {
	return &InquiryHandler{uc: uc}
}

// List は `GET /internal/v1/inquiries?status=new,in_progress` を処理する。
func (h *InquiryHandler) List(c *gin.Context) {
	statusRaw := c.Query("status")
	var statuses []string
	if statusRaw != "" {
		statuses = strings.Split(statusRaw, ",")
	}

	items, err := h.uc.List(c.Request.Context(), statuses)
	if err != nil {
		slog.Warn("inquiry list failed", "status", statusRaw, "error", err)
		c.JSON(errorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, apisupport.InquiryListResponse{Inquiries: presenter.ToInquirySummaries(items)})
}

// GetDetail は `GET /internal/v1/inquiries/:inquiryId` を処理する。
func (h *InquiryHandler) GetDetail(c *gin.Context) {
	id, ok := parseInquiryID(c)
	if !ok {
		return
	}
	inq, err := h.uc.Get(c.Request.Context(), id)
	if err != nil {
		slog.Warn("inquiry get failed", "inquiry_id", id, "error", err)
		c.JSON(errorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, presenter.ToInquiryDetail(inq))
}

// UpdateStatus は `POST /internal/v1/inquiries/:inquiryId/status` を処理する。
func (h *InquiryHandler) UpdateStatus(c *gin.Context) {
	id, ok := parseInquiryID(c)
	if !ok {
		return
	}
	var req apisupport.UpdateInquiryStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	inq, err := h.uc.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		slog.Warn("inquiry update status failed", "inquiry_id", id, "error", err)
		c.JSON(errorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, presenter.ToInquiryDetail(inq))
}

// UpdateNote は `POST /internal/v1/inquiries/:inquiryId/note` を処理する。
func (h *InquiryHandler) UpdateNote(c *gin.Context) {
	id, ok := parseInquiryID(c)
	if !ok {
		return
	}
	var req apisupport.UpdateInquiryNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	inq, err := h.uc.UpdateNote(c.Request.Context(), id, req.InternalNote)
	if err != nil {
		slog.Warn("inquiry update note failed", "inquiry_id", id, "error", err)
		c.JSON(errorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, presenter.ToInquiryDetail(inq))
}

// parseInquiryID はパスパラメータから inquiry_id を取り出す。不正なら 404 を即返して false。
func parseInquiryID(c *gin.Context) (int64, bool) {
	idStr := c.Param("inquiryId")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "inquiry not found"})
		return 0, false
	}
	return id, true
}
