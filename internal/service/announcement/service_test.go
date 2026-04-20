package announcement_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/service/announcement"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

var fixedNow = time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

func nowFixed() time.Time { return fixedNow }

// 仕様 (FEATURE_SPEC §3.2 / §4): lang は必須・対応言語のみ許容・フォールバックなし。
// lang が妥当なときのみ repo が呼ばれる (早期 fail)。
func TestList_仕様_langバリデーション(t *testing.T) {
	cases := []struct {
		name          string
		lang          string
		wantErr       error
		wantCallCount int
	}{
		{
			name:          "ja",
			lang:          apisupport.LangJa,
			wantErr:       nil,
			wantCallCount: 1,
		},
		{
			name:          "en",
			lang:          apisupport.LangEn,
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
				ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]apisupport.AnnouncementSummary, error) {
					callCount++
					return []apisupport.AnnouncementSummary{}, nil
				},
			}
			_, err := announcement.New(repo, nowFixed).List(context.Background(), tc.lang)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCallCount, callCount)
		})
	}
}

// 仕様: List は repo に lang と now を透過し、結果をそのまま返す。
func TestList_仕様_repoへの透過とレスポンス透過(t *testing.T) {
	want := []apisupport.AnnouncementSummary{{AnnouncementID: 1, Type: apisupport.TypeInfo, Title: "T", PublishedAt: fixedNow}}
	var gotLang string
	var gotNow time.Time
	repo := &port.MockAnnouncementRepo{
		ListPublishedFn: func(_ context.Context, lang string, now time.Time) ([]apisupport.AnnouncementSummary, error) {
			gotLang = lang
			gotNow = now
			return want, nil
		},
	}
	got, err := announcement.New(repo, nowFixed).List(context.Background(), apisupport.LangEn)

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, apisupport.LangEn, gotLang)
	assert.Equal(t, fixedNow, gotNow)
}


// 仕様 (FEATURE_SPEC §5): GetDetail は lang 必須。repo の ErrNotFound は service 層 ErrNotFound にマップする。
// それ以外のエラーは透過する (握りつぶし禁止)。
func TestGetDetail_仕様_langバリデーションとエラー透過(t *testing.T) {
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
			lang:     apisupport.LangJa,
			repoErr:  nil,
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "en 成功",
			lang:     apisupport.LangEn,
			repoErr:  nil,
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "repo の not found は service の ErrNotFound にマップ",
			lang:     apisupport.LangJa,
			repoErr:  port.ErrNotFound,
			wantErr:  announcement.ErrNotFound,
			wantCall: true,
		},
		{
			name:     "repo の DB 障害を透過 (500 系)",
			lang:     apisupport.LangJa,
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
				GetPublishedDetailFn: func(_ context.Context, _ int64, _ string) (*apisupport.AnnouncementDetail, error) {
					called = true
					if tc.repoErr != nil {
						return nil, tc.repoErr
					}
					return &apisupport.AnnouncementDetail{AnnouncementID: 1}, nil
				},
			}
			_, err := announcement.New(repo, nowFixed).GetDetail(context.Background(), 1, tc.lang)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCall, called)
		})
	}
}
