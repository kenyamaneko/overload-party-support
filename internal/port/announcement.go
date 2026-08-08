package port

import (
	"context"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// AnnouncementQuerier は公開 API (gateway → :9009) 向けの読み取り専用 port。
type AnnouncementQuerier interface {
	// ListPublished は指定 lang の翻訳が存在し、かつ公開期間内のお知らせを published_at DESC 順で返す。
	ListPublished(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error)

	// GetPublishedDetail は ID で単一のお知らせを返す (公開期間外・下書きでも返す)。
	GetPublishedDetail(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error)
}
