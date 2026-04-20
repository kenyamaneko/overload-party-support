package port

import (
	"context"
	"time"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// MockAnnouncementRepo は AnnouncementQuerier / AnnouncementAdmin を両方実装するテスト用モック。
// 必要な Fn フィールドだけ埋めて使い、未設定のメソッド呼び出しは panic で意図しない呼び出しを検出する。
type MockAnnouncementRepo struct {
	ListPublishedFn      func(ctx context.Context, lang string, now time.Time) ([]apisupport.AnnouncementSummary, error)
	GetPublishedDetailFn func(ctx context.Context, announcementID int64, lang string) (*apisupport.AnnouncementDetail, error)
	ListAllFn            func(ctx context.Context, filter StatusFilter, now time.Time) ([]apisupport.AnnouncementWithTranslations, error)
	GetFn                func(ctx context.Context, announcementID int64) (*apisupport.AnnouncementWithTranslations, error)
	CreateFn             func(ctx context.Context, params CreateAnnouncementParams) (int64, error)
	UpdateFn             func(ctx context.Context, announcementID int64, params UpdateAnnouncementParams) error
	DeleteFn             func(ctx context.Context, announcementID int64) error
	UpsertTranslationFn  func(ctx context.Context, announcementID int64, lang, title, body string) error
}

var (
	_ AnnouncementQuerier = (*MockAnnouncementRepo)(nil)
	_ AnnouncementAdmin   = (*MockAnnouncementRepo)(nil)
)

func (m *MockAnnouncementRepo) ListPublished(ctx context.Context, lang string, now time.Time) ([]apisupport.AnnouncementSummary, error) {
	if m.ListPublishedFn == nil {
		panic("MockAnnouncementRepo.ListPublished called without Fn")
	}
	return m.ListPublishedFn(ctx, lang, now)
}

func (m *MockAnnouncementRepo) GetPublishedDetail(ctx context.Context, announcementID int64, lang string) (*apisupport.AnnouncementDetail, error) {
	if m.GetPublishedDetailFn == nil {
		panic("MockAnnouncementRepo.GetPublishedDetail called without Fn")
	}
	return m.GetPublishedDetailFn(ctx, announcementID, lang)
}

func (m *MockAnnouncementRepo) ListAll(ctx context.Context, filter StatusFilter, now time.Time) ([]apisupport.AnnouncementWithTranslations, error) {
	if m.ListAllFn == nil {
		panic("MockAnnouncementRepo.ListAll called without Fn")
	}
	return m.ListAllFn(ctx, filter, now)
}

func (m *MockAnnouncementRepo) Get(ctx context.Context, announcementID int64) (*apisupport.AnnouncementWithTranslations, error) {
	if m.GetFn == nil {
		panic("MockAnnouncementRepo.Get called without Fn")
	}
	return m.GetFn(ctx, announcementID)
}

func (m *MockAnnouncementRepo) Create(ctx context.Context, params CreateAnnouncementParams) (int64, error) {
	if m.CreateFn == nil {
		panic("MockAnnouncementRepo.Create called without Fn")
	}
	return m.CreateFn(ctx, params)
}

func (m *MockAnnouncementRepo) Update(ctx context.Context, announcementID int64, params UpdateAnnouncementParams) error {
	if m.UpdateFn == nil {
		panic("MockAnnouncementRepo.Update called without Fn")
	}
	return m.UpdateFn(ctx, announcementID, params)
}

func (m *MockAnnouncementRepo) Delete(ctx context.Context, announcementID int64) error {
	if m.DeleteFn == nil {
		panic("MockAnnouncementRepo.Delete called without Fn")
	}
	return m.DeleteFn(ctx, announcementID)
}

func (m *MockAnnouncementRepo) UpsertTranslation(ctx context.Context, announcementID int64, lang, title, body string) error {
	if m.UpsertTranslationFn == nil {
		panic("MockAnnouncementRepo.UpsertTranslation called without Fn")
	}
	return m.UpsertTranslationFn(ctx, announcementID, lang, title, body)
}
