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

// AnnouncementRepository はお知らせの永続装置への CRUD port。
type AnnouncementRepository interface {
	// List は state 絞り込みを適用した一覧を返す。state == nil は全件。
	List(ctx context.Context, state *string, now time.Time) ([]domain.AnnouncementWithTranslations, error)

	// GetWithTranslations は ID でお知らせ本体 + 全言語翻訳を返す。
	GetWithTranslations(ctx context.Context, announcementID int64) (*domain.AnnouncementWithTranslations, error)

	// Create は本体 + 翻訳群を同一 tx で INSERT し、採番された ID を返す。
	Create(ctx context.Context, params domain.CreateAnnouncementParams) (int64, error)

	// Update は本体属性 (type / published_at / expires_at) を更新する。翻訳は UpsertTranslation 経由。
	Update(ctx context.Context, announcementID int64, params domain.UpdateAnnouncementParams) error

	// Delete は記事行を削除する。翻訳は FK CASCADE で同時削除される。
	Delete(ctx context.Context, announcementID int64) error

	// UpsertTranslation は翻訳を INSERT / UPDATE する。
	UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error
}

