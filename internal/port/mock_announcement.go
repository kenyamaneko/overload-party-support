package port

import (
	"context"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// MockAnnouncementRepo は AnnouncementQuerier を実装するテスト用モック。
// 必要な Fn フィールドだけ埋めて使い、未設定のメソッド呼び出しは panic で意図しない呼び出しを検出する。
type MockAnnouncementRepo struct {
	ListPublishedFn      func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error)
	GetPublishedDetailFn func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error)
}

var _ AnnouncementQuerier = (*MockAnnouncementRepo)(nil)

func (m *MockAnnouncementRepo) ListPublished(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
	if m.ListPublishedFn == nil {
		panic("MockAnnouncementRepo.ListPublished called without Fn")
	}
	return m.ListPublishedFn(ctx, lang, now)
}

func (m *MockAnnouncementRepo) GetPublishedDetail(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
	if m.GetPublishedDetailFn == nil {
		panic("MockAnnouncementRepo.GetPublishedDetail called without Fn")
	}
	return m.GetPublishedDetailFn(ctx, announcementID, lang)
}
