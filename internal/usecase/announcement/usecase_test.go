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

func TestList(t *testing.T) {
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

func TestList_LangValidation(t *testing.T) {
	cases := []struct {
		name    string
		lang    string
		wantErr error
	}{
		{
			name:    "未指定",
			lang:    "",
			wantErr: announcement.ErrLangRequired,
		},
		{
			name:    "対応外 (fr)",
			lang:    "fr",
			wantErr: announcement.ErrUnsupportedLang,
		},
		{
			name:    "対応外 (大文字)",
			lang:    "JA",
			wantErr: announcement.ErrUnsupportedLang,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := announcement.New(&port.MockAnnouncementRepo{}, nowFixed).List(context.Background(), tc.lang)

			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestGetDetail(t *testing.T) {
	want := &domain.AnnouncementDetail{AnnouncementID: 1}
	var gotID int64
	var gotLang string
	repo := &port.MockAnnouncementRepo{
		GetPublishedDetailFn: func(_ context.Context, id int64, lang string) (*domain.AnnouncementDetail, error) {
			gotID = id
			gotLang = lang
			return want, nil
		},
	}
	got, err := announcement.New(repo, nowFixed).GetDetail(context.Background(), 1, domain.LangEn)

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, int64(1), gotID)
	assert.Equal(t, domain.LangEn, gotLang)
}

func TestGetDetail_RepoError(t *testing.T) {
	dbErr := errors.New("db connection lost")

	cases := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{
			name:    "port.ErrNotFound は ErrNotFound にマップ",
			repoErr: port.ErrNotFound,
			wantErr: announcement.ErrNotFound,
		},
		{
			name:    "それ以外のエラーは透過 (握りつぶし禁止)",
			repoErr: dbErr,
			wantErr: dbErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(_ context.Context, _ int64, _ string) (*domain.AnnouncementDetail, error) {
					return nil, tc.repoErr
				},
			}
			_, err := announcement.New(repo, nowFixed).GetDetail(context.Background(), 1, domain.LangJa)

			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestGetDetail_LangValidation(t *testing.T) {
	cases := []struct {
		name    string
		lang    string
		wantErr error
	}{
		{
			name:    "未指定",
			lang:    "",
			wantErr: announcement.ErrLangRequired,
		},
		{
			name:    "対応外 (fr)",
			lang:    "fr",
			wantErr: announcement.ErrUnsupportedLang,
		},
		{
			name:    "対応外 (大文字)",
			lang:    "JA",
			wantErr: announcement.ErrUnsupportedLang,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := announcement.New(&port.MockAnnouncementRepo{}, nowFixed).GetDetail(context.Background(), 1, tc.lang)

			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
