// Package announcement は公開お知らせ API (gateway → :9009) のユースケース層。
// lang 必須検証・フォールバックなし方針の強制と port.AnnouncementQuerier への委譲のみを担う。
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

// New は Usecase を構築する。now は時刻注入 (テスト時は固定時刻を渡せる)。
func New(querier port.AnnouncementQuerier, now func() time.Time) *Usecase {
	return &Usecase{querier: querier, now: now}
}

// List は指定 lang の公開中お知らせ一覧を返す (FEATURE_SPEC §4)。
// lang が空 or 対応外なら ErrLangRequired / ErrUnsupportedLang。
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

// GetDetail は ID 指定で詳細を返す (FEATURE_SPEC §5)。
// 期間外・下書きでも返す。翻訳未存在 or 記事未存在なら ErrNotFound。
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

// validateLang は §3.2 の言語解決契約を強制する。
// 必須・対応言語のいずれでもなければエラー。フォールバックは一切行わない。
func validateLang(lang string) error {
	if lang == "" {
		return ErrLangRequired
	}
	if !domain.IsSupportedLang(lang) {
		return ErrUnsupportedLang
	}
	return nil
}
