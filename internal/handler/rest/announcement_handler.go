package rest

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// AnnouncementHandler は gateway が呼び出すお知らせ読み取り API。
type AnnouncementHandler struct {
	uc *announcement.Usecase
}

// NewAnnouncementHandler は AnnouncementHandler を生成する。
func NewAnnouncementHandler(uc *announcement.Usecase) *AnnouncementHandler {
	return &AnnouncementHandler{uc: uc}
}

// List は `GET /internal/v1/announcements?lang=<code>` を処理する (FEATURE_SPEC §4)。
func (h *AnnouncementHandler) List(c *gin.Context) {
	lang := c.Query("lang")
	items, err := h.uc.List(c.Request.Context(), lang)
	if err != nil {
		slog.Warn("announcement list failed", "lang", lang, "error", err)
		c.JSON(errorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, apisupport.AnnouncementListResponse{Announcements: toAnnouncementSummaryResponses(items)})
}

// GetDetail は `GET /internal/v1/announcements/:announcementId?lang=<code>` を処理する (FEATURE_SPEC §5)。
func (h *AnnouncementHandler) GetDetail(c *gin.Context) {
	lang := c.Query("lang")

	idStr := c.Param("announcementId")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "announcement not found"})
		return
	}

	detail, err := h.uc.GetDetail(c.Request.Context(), id, lang)
	if err != nil {
		slog.Warn("announcement get detail failed", "announcement_id", id, "lang", lang, "error", err)
		c.JSON(errorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toAnnouncementDetailResponse(detail))
}

// toAnnouncementSummaryResponses は domain の AnnouncementSummary を REST wire の
// apisupport.AnnouncementSummary へ詰め替える。delivery 層の境界変換。
// nil / 空集合の場合は長さ 0 の slice を返す (JSON で `[]` を保証する契約)。
func toAnnouncementSummaryResponses(items []domain.AnnouncementSummary) []apisupport.AnnouncementSummary {
	out := make([]apisupport.AnnouncementSummary, 0, len(items))
	for _, it := range items {
		out = append(out, apisupport.AnnouncementSummary{
			AnnouncementID: it.AnnouncementID,
			Type:           it.Type,
			Title:          it.Title,
			PublishedAt:    it.PublishedAt,
		})
	}
	return out
}

// toAnnouncementDetailResponse は domain の AnnouncementDetail を REST wire の
// apisupport.AnnouncementDetail へ詰め替える。delivery 層の境界変換。
func toAnnouncementDetailResponse(d *domain.AnnouncementDetail) apisupport.AnnouncementDetail {
	return apisupport.AnnouncementDetail{
		AnnouncementID: d.AnnouncementID,
		Type:           d.Type,
		Title:          d.Title,
		Body:           d.Body,
		PublishedAt:    d.PublishedAt,
	}
}
