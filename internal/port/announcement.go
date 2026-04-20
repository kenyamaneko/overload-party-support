package port

import (
	"context"
	"time"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// AnnouncementQuerier は公開 API (gateway → :9009) 向けの読み取り専用 port。
// 実装は postgres 側で published_at / expires_at / 翻訳存在のフィルタを完結させる。
type AnnouncementQuerier interface {
	// ListPublished は指定 lang の翻訳が存在し、かつ公開期間内のお知らせを published_at DESC 順で返す。
	ListPublished(ctx context.Context, lang string, now time.Time) ([]apisupport.AnnouncementSummary, error)

	// GetPublishedDetail は ID で単一のお知らせを返す。公開期間外・下書きでも返す (FEATURE_SPEC §5.2)。
	// 記事行が存在しない or 指定 lang 翻訳が存在しない場合は ErrNotFound。
	GetPublishedDetail(ctx context.Context, announcementID int64, lang string) (*apisupport.AnnouncementDetail, error)
}

// AnnouncementAdmin は管理 UI (:9109) 向けの CRUD port。
// 下書きや公開期間外も含む全件を扱う。
type AnnouncementAdmin interface {
	// ListAll は status フィルタ (draft / scheduled / published / expired / all) を適用した一覧を返す。
	// 管理 UI は件数制限を持たないため limit は渡さない。
	ListAll(ctx context.Context, filter StatusFilter, now time.Time) ([]apisupport.AnnouncementWithTranslations, error)

	// Get は ID でお知らせを翻訳込みで返す。
	Get(ctx context.Context, announcementID int64) (*apisupport.AnnouncementWithTranslations, error)

	// Create は本体 + 翻訳群を同一 tx で INSERT し、採番された ID を返す (FEATURE_SPEC §6.4)。
	// どの言語が必須かは service 層で検証済み。repo は受け取った翻訳をそのまま INSERT する。
	Create(ctx context.Context, params CreateAnnouncementParams) (int64, error)

	// Update は本体属性 (type / published_at / expires_at) を更新する。翻訳は UpsertTranslation 経由。
	Update(ctx context.Context, announcementID int64, params UpdateAnnouncementParams) error

	// Delete は記事行を削除する。翻訳は FK CASCADE で同時削除される。
	Delete(ctx context.Context, announcementID int64) error

	// UpsertTranslation は翻訳を INSERT / UPDATE する。
	UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error
}

// StatusFilter は管理 UI の一覧絞り込みキー。
type StatusFilter string

const (
	StatusFilterAll       StatusFilter = "all"
	StatusFilterDraft     StatusFilter = "draft"
	StatusFilterScheduled StatusFilter = "scheduled"
	StatusFilterPublished StatusFilter = "published"
	StatusFilterExpired   StatusFilter = "expired"
)

// CreateAnnouncementParams は新規作成に必要なフィールド集合。
type CreateAnnouncementParams struct {
	Type         apisupport.Type
	PublishedAt  *time.Time // nil: 下書き
	ExpiresAt    *time.Time // nil: 無期限
	Translations []TranslationInput
}

// TranslationInput は Create 時に同一 tx で INSERT する翻訳 1 件分。
type TranslationInput struct {
	Lang  string
	Title string
	Body  string
}

// UpdateAnnouncementParams は本体更新に必要なフィールド集合。
type UpdateAnnouncementParams struct {
	Type        apisupport.Type
	PublishedAt *time.Time
	ExpiresAt   *time.Time
}
