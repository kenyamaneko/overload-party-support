package announcement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

// Usecase は公開お知らせ API のユースケースを担う。
type Usecase struct {
	querier port.AnnouncementQuerier
	now     func() time.Time
}

// New は Usecase を構築する。
func New(querier port.AnnouncementQuerier, now func() time.Time) *Usecase {
	return &Usecase{querier: querier, now: now}
}

// List は指定 lang の公開中お知らせ一覧を返す。
func (u *Usecase) List(ctx context.Context, lang string) ([]domain.AnnouncementSummary, error) {
	if err := validateLang(lang); err != nil {
		return nil, err
	}
	items, err := u.querier.ListPublished(ctx, lang, u.now())
	if err != nil {
		return nil, fmt.Errorf("list published: %w", err)
	}
	return items, nil
}

// GetDetail は ID 指定で詳細を返す (期間外・下書きでも返す)。
func (u *Usecase) GetDetail(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
	if err := validateLang(lang); err != nil {
		return nil, err
	}
	d, err := u.querier.GetPublishedDetail(ctx, announcementID, lang)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get detail: %w", err)
	}
	return d, nil
}

// validateLang は必須・対応言語のいずれでもなければエラー。
func validateLang(lang string) error {
	if lang == "" {
		return ErrLangRequired
	}
	if !domain.IsSupportedLang(lang) {
		return ErrUnsupportedLang
	}
	return nil
}
