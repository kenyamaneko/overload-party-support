package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres/postgrestest"
)

var pg *postgrestest.Postgres

func TestMain(m *testing.M) {
	os.Exit(postgrestest.RunMain(m, &pg))
}

// insertAnnouncement は support.announcements に1行挿入し、生成された announcement_id を返す。
func insertAnnouncement(t *testing.T, pool *pgxpool.Pool, announcementType string, publishedAt, expiresAt *time.Time) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO support.announcements (type, published_at, expires_at) VALUES ($1, $2, $3) RETURNING announcement_id`,
		announcementType, publishedAt, expiresAt,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertTranslation は support.announcement_translations に指定langの翻訳行を挿入する。
func insertTranslation(t *testing.T, pool *pgxpool.Pool, announcementID int64, lang, title, body string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO support.announcement_translations (announcement_id, lang, title, body) VALUES ($1, $2, $3, $4)`,
		announcementID, lang, title, body,
	)
	require.NoError(t, err)
}

func TestListPublished(t *testing.T) {
	t.Run("[お知らせリポジトリ]公開中お知らせ一覧の取得", func(t *testing.T) {
		now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
		past := now.Add(-1 * time.Hour)
		future := now.Add(1 * time.Hour)

		inclusionTests := []struct {
			name        string
			publishedAt *time.Time
			expiresAt   *time.Time
		}{
			{
				name:        "公開日時が現在時刻以前で、かつ期限日時が現在時刻より後で、指定langの翻訳が存在する行が結果に含まれる",
				publishedAt: &past,
				expiresAt:   &future,
			},
			{
				name:        "期限日時が未設定の行は、期限切れとして除外されない",
				publishedAt: &past,
				expiresAt:   nil,
			},
			{
				name:        "公開日時が現在時刻と同時刻の行は結果に含まれる",
				publishedAt: &now,
				expiresAt:   nil,
			},
		}
		for _, tt := range inclusionTests {
			t.Run(tt.name, func(t *testing.T) {
				pg.Truncate(t)
				repo := postgres.NewAnnouncementRepository(pg.Pool)
				id := insertAnnouncement(t, pg.Pool, "info", tt.publishedAt, tt.expiresAt)
				insertTranslation(t, pg.Pool, id, "ja", "タイトル", "本文")

				got, err := repo.ListPublished(context.Background(), "ja", now)

				require.NoError(t, err)
				require.Len(t, got, 1)
				assert.Equal(t, id, got[0].AnnouncementID)
			})
		}

		exclusionTests := []struct {
			name            string
			publishedAt     *time.Time
			expiresAt       *time.Time
			translationLang string
		}{
			{
				name:            "公開日時が未設定の行は結果に含まれない",
				publishedAt:     nil,
				expiresAt:       nil,
				translationLang: "ja",
			},
			{
				name:            "公開日時が現在時刻より後(公開前)の行は結果に含まれない",
				publishedAt:     &future,
				expiresAt:       nil,
				translationLang: "ja",
			},
			{
				name:            "期限日時が現在時刻以前(期限日時と現在時刻が同時刻の場合を含む)の行は結果に含まれない",
				publishedAt:     &past,
				expiresAt:       &now,
				translationLang: "ja",
			},
			{
				name:            "指定langの翻訳が存在しない行は結果に含まれない(他langの翻訳しかない行は対象外になる)",
				publishedAt:     &past,
				expiresAt:       nil,
				translationLang: "en",
			},
		}
		for _, tt := range exclusionTests {
			t.Run(tt.name, func(t *testing.T) {
				pg.Truncate(t)
				repo := postgres.NewAnnouncementRepository(pg.Pool)
				id := insertAnnouncement(t, pg.Pool, "info", tt.publishedAt, tt.expiresAt)
				insertTranslation(t, pg.Pool, id, tt.translationLang, "タイトル", "本文")

				got, err := repo.ListPublished(context.Background(), "ja", now)

				require.NoError(t, err)
				assert.Empty(t, got)
			})
		}

		t.Run("公開条件を満たす行が複数あるとき、公開日時の降順で並ぶ", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			older := now.Add(-2 * time.Hour)
			newer := now.Add(-1 * time.Hour)
			olderID := insertAnnouncement(t, pg.Pool, "info", &older, nil)
			insertTranslation(t, pg.Pool, olderID, "ja", "古い", "本文")
			newerID := insertAnnouncement(t, pg.Pool, "info", &newer, nil)
			insertTranslation(t, pg.Pool, newerID, "ja", "新しい", "本文")

			got, err := repo.ListPublished(context.Background(), "ja", now)

			require.NoError(t, err)
			require.Len(t, got, 2)
			assert.Equal(t, []int64{newerID, olderID}, []int64{got[0].AnnouncementID, got[1].AnnouncementID})
		})

		t.Run("公開日時が同じ行同士は、announcement_idの降順で並ぶ", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			firstID := insertAnnouncement(t, pg.Pool, "info", &past, nil)
			insertTranslation(t, pg.Pool, firstID, "ja", "先に登録", "本文")
			secondID := insertAnnouncement(t, pg.Pool, "info", &past, nil)
			insertTranslation(t, pg.Pool, secondID, "ja", "後に登録", "本文")

			got, err := repo.ListPublished(context.Background(), "ja", now)

			require.NoError(t, err)
			require.Len(t, got, 2)
			assert.Equal(t, []int64{secondID, firstID}, []int64{got[0].AnnouncementID, got[1].AnnouncementID})
		})

		t.Run("該当する行が無いとき、要素数0の結果を返す", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)

			got, err := repo.ListPublished(context.Background(), "ja", now)

			require.NoError(t, err)
			assert.Empty(t, got)
		})
	})
}

func TestGetPublishedDetail(t *testing.T) {
	t.Run("[お知らせリポジトリ]お知らせ詳細の取得", func(t *testing.T) {
		now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
		past := now.Add(-1 * time.Hour)
		future := now.Add(1 * time.Hour)

		t.Run("指定announcementIdの行に指定langの翻訳が存在するとき、公開日時が未設定(下書き)でも取得できる", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			id := insertAnnouncement(t, pg.Pool, "info", nil, nil)
			insertTranslation(t, pg.Pool, id, "ja", "タイトル", "本文")

			got, err := repo.GetPublishedDetail(context.Background(), id, "ja")

			require.NoError(t, err)
			assert.Equal(t, id, got.AnnouncementID)
		})

		t.Run("指定announcementIdの行に指定langの翻訳が存在するとき、公開日時が現在時刻より後(公開前)でも取得できる", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			id := insertAnnouncement(t, pg.Pool, "info", &future, nil)
			insertTranslation(t, pg.Pool, id, "ja", "タイトル", "本文")

			got, err := repo.GetPublishedDetail(context.Background(), id, "ja")

			require.NoError(t, err)
			assert.Equal(t, id, got.AnnouncementID)
		})

		t.Run("指定announcementIdの行に指定langの翻訳が存在するとき、公開日時が現在時刻以前(公開中)でも取得できる", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			id := insertAnnouncement(t, pg.Pool, "info", &past, nil)
			insertTranslation(t, pg.Pool, id, "ja", "タイトル", "本文")

			got, err := repo.GetPublishedDetail(context.Background(), id, "ja")

			require.NoError(t, err)
			assert.Equal(t, id, got.AnnouncementID)
		})

		t.Run("指定announcementIdの行に指定langの翻訳が存在するとき、期限日時が現在時刻以前(期限切れ)でも取得できる", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			id := insertAnnouncement(t, pg.Pool, "info", &past, &past)
			insertTranslation(t, pg.Pool, id, "ja", "タイトル", "本文")

			got, err := repo.GetPublishedDetail(context.Background(), id, "ja")

			require.NoError(t, err)
			assert.Equal(t, id, got.AnnouncementID)
		})

		t.Run("指定announcementIdの行が存在しないとき、port.ErrNotFoundを返す", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)

			_, err := repo.GetPublishedDetail(context.Background(), 1, "ja")

			assert.ErrorIs(t, err, port.ErrNotFound)
		})

		t.Run("指定announcementIdの行は存在するが指定langの翻訳が存在しないとき、port.ErrNotFoundを返す", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			id := insertAnnouncement(t, pg.Pool, "info", &past, nil)
			insertTranslation(t, pg.Pool, id, "en", "title", "body")

			_, err := repo.GetPublishedDetail(context.Background(), id, "ja")

			assert.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}
