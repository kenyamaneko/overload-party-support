package announcement_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
)

var fixedNow = time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

func nowFixed() time.Time { return fixedNow }

// 仕様 (FEATURE_SPEC §3.2 / §4): lang はフォールバックせずエラー。妥当なときのみ repo が呼ばれる (早期 fail)。
func TestList_LangValidation(t *testing.T) {
	cases := []struct {
		name          string
		lang          string
		wantErr       error
		wantCallCount int
	}{
		{
			name:          "ja",
			lang:          domain.LangJa,
			wantErr:       nil,
			wantCallCount: 1,
		},
		{
			name:          "en",
			lang:          domain.LangEn,
			wantErr:       nil,
			wantCallCount: 1,
		},
		{
			name:          "未指定",
			lang:          "",
			wantErr:       announcement.ErrLangRequired,
			wantCallCount: 0,
		},
		{
			name:          "対応外 (fr)",
			lang:          "fr",
			wantErr:       announcement.ErrUnsupportedLang,
			wantCallCount: 0,
		},
		{
			name:          "対応外 (大文字)",
			lang:          "JA",
			wantErr:       announcement.ErrUnsupportedLang,
			wantCallCount: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var callCount int
			repo := &port.MockAnnouncementRepo{
				ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]domain.AnnouncementSummary, error) {
					callCount++
					return []domain.AnnouncementSummary{}, nil
				},
			}
			_, err := announcement.New(repo, nowFixed).List(context.Background(), tc.lang)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCallCount, callCount)
		})
	}
}

// 仕様: List は repo に lang と now を透過し、結果をそのまま返す。
func TestList_PortPassthrough(t *testing.T) {
	want := []domain.AnnouncementSummary{{AnnouncementID: 1, Type: domain.TypeInfo, Title: "T", PublishedAt: fixedNow}}
	var gotLang string
	var gotNow time.Time
	repo := &port.MockAnnouncementRepo{
		ListPublishedFn: func(_ context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
			gotLang = lang
			gotNow = now
			return want, nil
		},
	}
	got, err := announcement.New(repo, nowFixed).List(context.Background(), domain.LangEn)

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, domain.LangEn, gotLang)
	assert.Equal(t, fixedNow, gotNow)
}


// 仕様 (FEATURE_SPEC §5): GetDetail は repo の ErrNotFound を usecase の ErrNotFound にマップ。それ以外は透過 (握りつぶし禁止)。
func TestGetDetail(t *testing.T) {
	dbErr := errors.New("db connection lost")

	cases := []struct {
		name     string
		lang     string
		repoErr  error
		wantErr  error
		wantCall bool
	}{
		{
			name:     "ja 成功",
			lang:     domain.LangJa,
			repoErr:  nil,
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "en 成功",
			lang:     domain.LangEn,
			repoErr:  nil,
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "repo の not found は usecase の ErrNotFound にマップ",
			lang:     domain.LangJa,
			repoErr:  port.ErrNotFound,
			wantErr:  announcement.ErrNotFound,
			wantCall: true,
		},
		{
			name:     "repo の DB 障害を透過 (500 系)",
			lang:     domain.LangJa,
			repoErr:  dbErr,
			wantErr:  dbErr,
			wantCall: true,
		},
		{
			name:     "lang 未指定",
			lang:     "",
			repoErr:  nil,
			wantErr:  announcement.ErrLangRequired,
			wantCall: false,
		},
		{
			name:     "lang 対応外",
			lang:     "fr",
			repoErr:  nil,
			wantErr:  announcement.ErrUnsupportedLang,
			wantCall: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			repo := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(_ context.Context, _ int64, _ string) (*domain.AnnouncementDetail, error) {
					called = true
					if tc.repoErr != nil {
						return nil, tc.repoErr
					}
					return &domain.AnnouncementDetail{AnnouncementID: 1}, nil
				},
			}
			_, err := announcement.New(repo, nowFixed).GetDetail(context.Background(), 1, tc.lang)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCall, called)
		})
	}
}
