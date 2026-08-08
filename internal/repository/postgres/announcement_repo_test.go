package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres/postgrestest"
)

var sharedPG *postgrestest.Postgres

func TestMain(m *testing.M) {
	os.Exit(postgrestest.RunMain(m, &sharedPG))
}

var fixedNow = time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

// newAnnouncementRepo は共有 Postgres を TRUNCATE した上で repo を生成する。
func newAnnouncementRepo(t *testing.T) *postgres.AnnouncementRepository {
	t.Helper()
	sharedPG.Truncate(t)
	return postgres.NewAnnouncementRepository(sharedPG.Pool)
}

// translationSeed は SQL 直挿しフィクスチャの翻訳 1 件分。
type translationSeed struct {
	lang  string
	title string
	body  string
}

// jaSeed は ja 1 件のみの翻訳シード (多くの seed で使うのでヘルパー化)。
func jaSeed(title, body string) []translationSeed {
	return []translationSeed{{lang: domain.LangJa, title: title, body: body}}
}

// insertAnnouncement は support.announcements + announcement_translations へ直接 INSERT し、採番された announcement_id を返す。
func insertAnnouncement(t *testing.T, typ string, publishedAt, expiresAt *time.Time, translations []translationSeed) int64 {
	t.Helper()
	ctx := context.Background()

	var id int64
	require.NoError(t, sharedPG.Pool.QueryRow(ctx,
		`INSERT INTO support.announcements (type, published_at, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING announcement_id`,
		typ, publishedAt, expiresAt,
	).Scan(&id))

	for _, tr := range translations {
		_, err := sharedPG.Pool.Exec(ctx,
			`INSERT INTO support.announcement_translations (announcement_id, lang, title, body)
			 VALUES ($1, $2, $3, $4)`,
			id, tr.lang, tr.title, tr.body,
		)
		require.NoError(t, err)
	}
	return id
}

// announcementSeed は複数件を一括で INSERT するときの 1 件分。
type announcementSeed struct {
	typ          string
	publishedAt  *time.Time
	expiresAt    *time.Time
	translations []translationSeed
}

func TestListPublished(t *testing.T) {
	t.Run("公開告知一覧の取得", func(t *testing.T) {
		t.Run("公開条件を満たす行だけがlang別に返る", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			past := fixedNow.Add(-24 * time.Hour)
			future := fixedNow.Add(24 * time.Hour)
			farPast := fixedNow.Add(-48 * time.Hour)

			seeds := []announcementSeed{
				{
					typ:         domain.TypeInfo,
					publishedAt: &past,
					translations: []translationSeed{
						{lang: domain.LangJa, title: "published_at<=now (ja+en)", body: "B"},
						{lang: domain.LangEn, title: "published_at<=now (ja+en) [en]", body: "B-en"},
					},
				},
				{
					typ:          domain.TypeInfo,
					publishedAt:  nil,
					translations: jaSeed("published_at=NULL", "B"),
				},
				{
					typ:          domain.TypeInfo,
					publishedAt:  &future,
					translations: jaSeed("published_at>now", "B"),
				},
				{
					typ:          domain.TypeEvent,
					publishedAt:  &farPast,
					expiresAt:    &past,
					translations: jaSeed("expires_at<=now", "B"),
				},
				{
					typ:          domain.TypeInfo,
					publishedAt:  &past,
					expiresAt:    &future,
					translations: jaSeed("expires_at>now", "B"),
				},
				{
					typ:          domain.TypeInfo,
					publishedAt:  &past,
					translations: jaSeed("published_at<=now (ja only)", "B"),
				},
				{
					typ:          domain.TypeInfo,
					publishedAt:  &fixedNow,
					translations: jaSeed("published_at==now", "B"),
				},
				{
					typ:          domain.TypeInfo,
					publishedAt:  &farPast,
					expiresAt:    &fixedNow,
					translations: jaSeed("expires_at==now", "B"),
				},
			}
			for _, s := range seeds {
				insertAnnouncement(t, s.typ, s.publishedAt, s.expiresAt, s.translations)
			}

			cases := []struct {
				name       string
				lang       string
				wantTitles []string
			}{
				{
					name: "lang=jaのとき、公開条件を満たす行 (ja+en行 / ja only行 / expires_at>now行 / published_at==now行)を返す (expires_at==nowは含まない)",
					lang: domain.LangJa,
					wantTitles: []string{
						"published_at<=now (ja+en)",
						"published_at<=now (ja only)",
						"expires_at>now",
						"published_at==now",
					},
				},
				{
					name: "lang=enのとき、公開条件を満たしen翻訳もある1件のみ返す",
					lang: domain.LangEn,
					wantTitles: []string{
						"published_at<=now (ja+en) [en]",
					},
				},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					items, err := repo.ListPublished(ctx, tc.lang, fixedNow)
					require.NoError(t, err)

					titles := make([]string, 0, len(items))
					for _, it := range items {
						titles = append(titles, it.Title)
					}
					assert.ElementsMatch(t, tc.wantTitles, titles)
				})
			}
		})

		t.Run("空DBのとき、nilでなく長さ0のsliceを返す", func(t *testing.T) {
			// 呼び出し側が range できる契約のため、空でも nil を返さない。
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			items, err := repo.ListPublished(ctx, domain.LangJa, fixedNow)
			require.NoError(t, err)
			assert.Empty(t, items)
		})

		t.Run("published_at DESC・announcement_id DESCで並ぶ", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			t1 := fixedNow.Add(-3 * time.Hour)
			t2 := fixedNow.Add(-2 * time.Hour)
			t3 := fixedNow.Add(-1 * time.Hour)

			id1a := insertAnnouncement(t, domain.TypeInfo, &t1, nil, jaSeed("1a", "B"))
			id1b := insertAnnouncement(t, domain.TypeInfo, &t1, nil, jaSeed("1b", "B"))
			id2 := insertAnnouncement(t, domain.TypeInfo, &t2, nil, jaSeed("2", "B"))
			id3 := insertAnnouncement(t, domain.TypeInfo, &t3, nil, jaSeed("3", "B"))

			items, err := repo.ListPublished(ctx, domain.LangJa, fixedNow)
			require.NoError(t, err)
			require.Len(t, items, 4)

			gotIDs := []int64{items[0].AnnouncementID, items[1].AnnouncementID, items[2].AnnouncementID, items[3].AnnouncementID}
			assert.Equal(t, []int64{id3, id2, id1b, id1a}, gotIDs)
		})
	})
}

func TestGetPublishedDetail(t *testing.T) {
	t.Run("公開告知詳細の取得", func(t *testing.T) {
		t.Run("published_atの値に依存せず翻訳行があれば返る", func(t *testing.T) {
			// 公開期間外でもアクセス可 (published_at に依存しない)。
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			past := fixedNow.Add(-time.Hour)
			future := fixedNow.Add(time.Hour)

			pastID := insertAnnouncement(t, domain.TypeInfo, &past, nil, jaSeed("past", "B-past"))
			nullID := insertAnnouncement(t, domain.TypeInfo, nil, nil, jaSeed("null", "B-null"))
			futureID := insertAnnouncement(t, domain.TypeInfo, &future, nil, jaSeed("future", "B-future"))

			cases := []struct {
				name     string
				id       int64
				wantBody string
			}{
				{
					name:     "published_at<=nowのとき、bodyを返す",
					id:       pastID,
					wantBody: "B-past",
				},
				{
					name:     "published_at=NULLのとき、bodyを返す",
					id:       nullID,
					wantBody: "B-null",
				},
				{
					name:     "published_at>nowのとき、bodyを返す",
					id:       futureID,
					wantBody: "B-future",
				},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					detail, err := repo.GetPublishedDetail(ctx, tc.id, domain.LangJa)
					require.NoError(t, err)
					require.NotNil(t, detail)
					assert.Equal(t, tc.wantBody, detail.Body)
				})
			}
		})

		t.Run("対象が無いとき、ErrNotFoundになりdetailはnil", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			past := fixedNow.Add(-time.Hour)
			jaOnlyID := insertAnnouncement(t, domain.TypeInfo, &past, nil, jaSeed("ja only", "B"))

			cases := []struct {
				name string
				id   int64
				lang string
			}{
				{
					name: "指定langの翻訳行が無い (jaのみ存在する行にenで問い合わせ)とき、ErrNotFoundになる",
					id:   jaOnlyID,
					lang: domain.LangEn,
				},
				{
					name: "announcement_idの行が無いとき、ErrNotFoundになる",
					id:   999999,
					lang: domain.LangJa,
				},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					detail, err := repo.GetPublishedDetail(ctx, tc.id, tc.lang)
					assert.ErrorIs(t, err, port.ErrNotFound)
					assert.Nil(t, detail)
				})
			}
		})
	})
}
