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

var errFromQuerier = errors.New("querier: boom")

func TestList(t *testing.T) {
	t.Run("[公開お知らせユースケース]公開お知らせ一覧の取得", func(t *testing.T) {
		t.Run("langが空文字のとき、announcement.ErrLangRequiredを返す", func(t *testing.T) {
			uc := announcement.New(&port.MockAnnouncementRepo{}, time.Now)

			_, err := uc.List(context.Background(), "")

			assert.ErrorIs(t, err, announcement.ErrLangRequired)
		})

		t.Run("langが対応外の値のとき、announcement.ErrUnsupportedLangを返す", func(t *testing.T) {
			uc := announcement.New(&port.MockAnnouncementRepo{}, time.Now)

			_, err := uc.List(context.Background(), "fr")

			assert.ErrorIs(t, err, announcement.ErrUnsupportedLang)
		})

		t.Run("langが対応言語のとき、指定したlangと現在時刻をお知らせ取得ポートへ渡す", func(t *testing.T) {
			fixedNow := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			var gotLang string
			var gotNow time.Time
			querier := &port.MockAnnouncementRepo{
				ListPublishedFn: func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
					gotLang = lang
					gotNow = now
					return nil, nil
				},
			}
			uc := announcement.New(querier, func() time.Time { return fixedNow })

			_, err := uc.List(context.Background(), "ja")

			require.NoError(t, err)
			assert.Equal(t, "ja", gotLang)
			assert.True(t, fixedNow.Equal(gotNow))
		})

		t.Run("お知らせ取得ポートが結果を返したとき、その結果をそのまま呼び出し元に返す", func(t *testing.T) {
			want := []domain.AnnouncementSummary{
				{AnnouncementID: 1, Type: domain.TypeInfo, Title: "タイトル", PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			}
			querier := &port.MockAnnouncementRepo{
				ListPublishedFn: func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
					return want, nil
				},
			}
			uc := announcement.New(querier, time.Now)

			got, err := uc.List(context.Background(), "ja")

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})

		t.Run("お知らせ取得ポートが想定外のエラーを返したとき、そのエラーを握りつぶさず、その原因のエラーだと判別できる形で呼び出し元に伝播する", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				ListPublishedFn: func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
					return nil, errFromQuerier
				},
			}
			uc := announcement.New(querier, time.Now)

			_, err := uc.List(context.Background(), "ja")

			assert.ErrorIs(t, err, errFromQuerier)
		})
	})
}

func TestGetDetail(t *testing.T) {
	t.Run("[公開お知らせユースケース]公開お知らせ詳細の取得", func(t *testing.T) {
		t.Run("langが空文字のとき、announcement.ErrLangRequiredを返す", func(t *testing.T) {
			uc := announcement.New(&port.MockAnnouncementRepo{}, time.Now)

			_, err := uc.GetDetail(context.Background(), 1, "")

			assert.ErrorIs(t, err, announcement.ErrLangRequired)
		})

		t.Run("langが対応外の値のとき、announcement.ErrUnsupportedLangを返す", func(t *testing.T) {
			uc := announcement.New(&port.MockAnnouncementRepo{}, time.Now)

			_, err := uc.GetDetail(context.Background(), 1, "fr")

			assert.ErrorIs(t, err, announcement.ErrUnsupportedLang)
		})

		t.Run("langが対応言語のとき、指定したannouncementIDとlangをお知らせ取得ポートへ渡す", func(t *testing.T) {
			var gotID int64
			var gotLang string
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					gotID = announcementID
					gotLang = lang
					return &domain.AnnouncementDetail{AnnouncementID: announcementID}, nil
				},
			}
			uc := announcement.New(querier, time.Now)

			_, err := uc.GetDetail(context.Background(), 42, "en")

			require.NoError(t, err)
			assert.Equal(t, int64(42), gotID)
			assert.Equal(t, "en", gotLang)
		})

		t.Run("お知らせ取得ポートが結果を返したとき、その結果をそのまま呼び出し元に返す", func(t *testing.T) {
			publishedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			want := &domain.AnnouncementDetail{
				AnnouncementID: 1,
				Type:           domain.TypeInfo,
				Title:          "タイトル",
				Body:           "本文",
				PublishedAt:    &publishedAt,
			}
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return want, nil
				},
			}
			uc := announcement.New(querier, time.Now)

			got, err := uc.GetDetail(context.Background(), 1, "ja")

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})

		t.Run("お知らせ取得ポートがport.ErrNotFoundを返したとき、announcement.ErrNotFoundに変換して返す", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return nil, port.ErrNotFound
				},
			}
			uc := announcement.New(querier, time.Now)

			_, err := uc.GetDetail(context.Background(), 1, "ja")

			assert.ErrorIs(t, err, announcement.ErrNotFound)
		})

		t.Run("お知らせ取得ポートがport.ErrNotFound以外の想定外のエラーを返したとき、そのエラーを握りつぶさず、その原因のエラーだと判別できる形で呼び出し元に伝播する", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return nil, errFromQuerier
				},
			}
			uc := announcement.New(querier, time.Now)

			_, err := uc.GetDetail(context.Background(), 1, "ja")

			assert.ErrorIs(t, err, errFromQuerier)
		})
	})
}
