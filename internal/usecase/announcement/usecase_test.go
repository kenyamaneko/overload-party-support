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
	t.Run("公開告知一覧の取得", func(t *testing.T) {
		t.Run("lang と now を repo に渡し、結果をそのまま返す", func(t *testing.T) {
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
		})

		t.Run("一覧の取得元が想定外のエラーを返すとき、握りつぶさずそのまま透過される", func(t *testing.T) {
			dbErr := errors.New("db lost")
			repo := &port.MockAnnouncementRepo{
				ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]domain.AnnouncementSummary, error) {
					return nil, dbErr
				},
			}
			_, err := announcement.New(repo, nowFixed).List(context.Background(), domain.LangJa)

			assert.ErrorIs(t, err, dbErr)
		})

		langCases := []struct {
			name    string
			lang    string
			wantErr error
		}{
			{
				name:    "lang が未指定のとき、ErrLangRequired になる",
				lang:    "",
				wantErr: announcement.ErrLangRequired,
			},
			{
				name:    "lang が対応外 (fr) のとき、ErrUnsupportedLang になる",
				lang:    "fr",
				wantErr: announcement.ErrUnsupportedLang,
			},
			{
				name:    "lang が大文字 (JA) のとき、ErrUnsupportedLang になる",
				lang:    "JA",
				wantErr: announcement.ErrUnsupportedLang,
			},
		}

		for _, tc := range langCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := announcement.New(&port.MockAnnouncementRepo{}, nowFixed).List(context.Background(), tc.lang)

				assert.ErrorIs(t, err, tc.wantErr)
			})
		}
	})
}

func TestGetDetail(t *testing.T) {
	t.Run("公開告知詳細の取得", func(t *testing.T) {
		t.Run("id と lang を repo に渡し、結果をそのまま返す", func(t *testing.T) {
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
		})

		dbErr := errors.New("db connection lost")
		repoErrCases := []struct {
			name    string
			repoErr error
			wantErr error
		}{
			{
				name:    "repo が port.ErrNotFound を返すとき、ErrNotFound にマップされる",
				repoErr: port.ErrNotFound,
				wantErr: announcement.ErrNotFound,
			},
			{
				name:    "repo がその他のエラーを返すとき、握りつぶさずそのまま透過される",
				repoErr: dbErr,
				wantErr: dbErr,
			},
		}

		for _, tc := range repoErrCases {
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

		langCases := []struct {
			name    string
			lang    string
			wantErr error
		}{
			{
				name:    "lang が未指定のとき、ErrLangRequired になる",
				lang:    "",
				wantErr: announcement.ErrLangRequired,
			},
			{
				name:    "lang が対応外 (fr) のとき、ErrUnsupportedLang になる",
				lang:    "fr",
				wantErr: announcement.ErrUnsupportedLang,
			},
			{
				name:    "lang が大文字 (JA) のとき、ErrUnsupportedLang になる",
				lang:    "JA",
				wantErr: announcement.ErrUnsupportedLang,
			},
		}

		for _, tc := range langCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := announcement.New(&port.MockAnnouncementRepo{}, nowFixed).GetDetail(context.Background(), 1, tc.lang)

				assert.ErrorIs(t, err, tc.wantErr)
			})
		}
	})
}
