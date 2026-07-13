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

// jaTr は ja 1 件のみの翻訳スライス (多くの seed で使うのでヘルパー化)。
func jaTr(title, body string) []domain.TranslationInput {
	return []domain.TranslationInput{{Lang: domain.LangJa, Title: title, Body: body}}
}

func TestCreate(t *testing.T) {
	t.Run("告知の作成", func(t *testing.T) {
		past := fixedNow.Add(-time.Hour)

		// repo はロジックを持たず素通すため、ja 必須などの業務制約 (usecase の責務) は課さず翻訳 0 件でも本体行を作る。
		cases := []struct {
			name            string
			params          domain.CreateAnnouncementParams
			wantTitleByLang map[string]string
		}{
			{
				name: "翻訳が 0 件でも本体行が作られる",
				params: domain.CreateAnnouncementParams{
					Type:         domain.TypeInfo,
					PublishedAt:  &past,
					Translations: nil,
				},
				wantTitleByLang: map[string]string{},
			},
			{
				name: "ja のみのとき、ja 翻訳が保存される",
				params: domain.CreateAnnouncementParams{
					Type:         domain.TypeInfo,
					PublishedAt:  &past,
					Translations: jaTr("T", "B"),
				},
				wantTitleByLang: map[string]string{domain.LangJa: "T"},
			},
			{
				name: "ja と en のとき、両翻訳が同一 tx で保存される",
				params: domain.CreateAnnouncementParams{
					Type:        domain.TypeInfo,
					PublishedAt: &past,
					Translations: []domain.TranslationInput{
						{Lang: domain.LangJa, Title: "T-ja", Body: "B-ja"},
						{Lang: domain.LangEn, Title: "T-en", Body: "B-en"},
					},
				},
				wantTitleByLang: map[string]string{domain.LangJa: "T-ja", domain.LangEn: "T-en"},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repo := newAnnouncementRepo(t)
				ctx := context.Background()

				id, err := repo.Create(ctx, tc.params)
				require.NoError(t, err)
				assert.Greater(t, id, int64(0))

				aw, err := repo.GetWithTranslations(ctx, id)
				require.NoError(t, err)
				require.NotNil(t, aw)
				assert.Equal(t, tc.params.Type, aw.Announcement.Type)

				gotTitleByLang := make(map[string]string, len(aw.Translations))
				for _, tr := range aw.Translations {
					gotTitleByLang[tr.Lang] = tr.Title
				}
				assert.Equal(t, tc.wantTitleByLang, gotTitleByLang)
			})
		}
	})
}

func TestListPublished(t *testing.T) {
	t.Run("公開告知一覧の取得", func(t *testing.T) {
		t.Run("公開条件を満たす行だけが lang 別に返る", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			past := fixedNow.Add(-24 * time.Hour)
			future := fixedNow.Add(24 * time.Hour)
			farPast := fixedNow.Add(-48 * time.Hour)

			seeds := []domain.CreateAnnouncementParams{
				{
					Type:        domain.TypeInfo,
					PublishedAt: &past,
					Translations: []domain.TranslationInput{
						{Lang: domain.LangJa, Title: "published_at<=now (ja+en)", Body: "B"},
						{Lang: domain.LangEn, Title: "published_at<=now (ja+en) [en]", Body: "B-en"},
					},
				},
				{
					Type:         domain.TypeInfo,
					PublishedAt:  nil,
					Translations: jaTr("published_at=NULL", "B"),
				},
				{
					Type:         domain.TypeInfo,
					PublishedAt:  &future,
					Translations: jaTr("published_at>now", "B"),
				},
				{
					Type:         domain.TypeEvent,
					PublishedAt:  &farPast,
					ExpiresAt:    &past,
					Translations: jaTr("expires_at<=now", "B"),
				},
				{
					Type:         domain.TypeInfo,
					PublishedAt:  &past,
					ExpiresAt:    &future,
					Translations: jaTr("expires_at>now", "B"),
				},
				{
					Type:         domain.TypeInfo,
					PublishedAt:  &past,
					Translations: jaTr("published_at<=now (ja only)", "B"),
				},
				{
					Type:         domain.TypeInfo,
					PublishedAt:  &fixedNow,
					Translations: jaTr("published_at==now", "B"),
				},
				{
					Type:         domain.TypeInfo,
					PublishedAt:  &farPast,
					ExpiresAt:    &fixedNow,
					Translations: jaTr("expires_at==now", "B"),
				},
			}
			for _, p := range seeds {
				_, err := repo.Create(ctx, p)
				require.NoError(t, err)
			}

			cases := []struct {
				name       string
				lang       string
				wantTitles []string
			}{
				{
					name: "lang=ja のとき、公開条件を満たす行 (ja+en 行 / ja only 行 / expires_at>now 行 / published_at==now 行) を返す (expires_at==now は含まない)",
					lang: domain.LangJa,
					wantTitles: []string{
						"published_at<=now (ja+en)",
						"published_at<=now (ja only)",
						"expires_at>now",
						"published_at==now",
					},
				},
				{
					name: "lang=en のとき、公開条件を満たし en 翻訳もある 1 件のみ返す",
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

		t.Run("空 DB のとき、nil でなく長さ 0 の slice を返す", func(t *testing.T) {
			// 呼び出し側が range できる契約のため、空でも nil を返さない。
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			items, err := repo.ListPublished(ctx, domain.LangJa, fixedNow)
			require.NoError(t, err)
			assert.Empty(t, items)
		})

		t.Run("published_at DESC・announcement_id DESC で並ぶ", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			t1 := fixedNow.Add(-3 * time.Hour)
			t2 := fixedNow.Add(-2 * time.Hour)
			t3 := fixedNow.Add(-1 * time.Hour)

			mustCreate := func(p domain.CreateAnnouncementParams) int64 {
				t.Helper()
				id, err := repo.Create(ctx, p)
				require.NoError(t, err)
				return id
			}
			id1a := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &t1, Translations: jaTr("1a", "B")})
			id1b := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &t1, Translations: jaTr("1b", "B")})
			id2 := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &t2, Translations: jaTr("2", "B")})
			id3 := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &t3, Translations: jaTr("3", "B")})

			items, err := repo.ListPublished(ctx, domain.LangJa, fixedNow)
			require.NoError(t, err)
			require.Len(t, items, 4)

			gotIDs := []int64{items[0].AnnouncementID, items[1].AnnouncementID, items[2].AnnouncementID, items[3].AnnouncementID}
			assert.Equal(t, []int64{id3, id2, id1b, id1a}, gotIDs)
		})
	})
}

func TestList(t *testing.T) {
	t.Run("state による絞り込み一覧", func(t *testing.T) {
		draft := domain.StateDraft
		scheduled := domain.StateScheduled
		published := domain.StatePublished
		expired := domain.StateExpired

		repo := newAnnouncementRepo(t)
		ctx := context.Background()

		past := fixedNow.Add(-time.Hour)
		future := fixedNow.Add(time.Hour)
		farPast := fixedNow.Add(-48 * time.Hour)

		seeds := []domain.CreateAnnouncementParams{
			{Type: domain.TypeInfo, PublishedAt: nil, Translations: jaTr("draft", "B")},
			{Type: domain.TypeInfo, PublishedAt: &future, Translations: jaTr("scheduled", "B")},
			{Type: domain.TypeInfo, PublishedAt: &past, Translations: jaTr("published", "B")},
			{Type: domain.TypeEvent, PublishedAt: &farPast, ExpiresAt: &past, Translations: jaTr("expired", "B")},
			{Type: domain.TypeInfo, PublishedAt: nil, ExpiresAt: &past, Translations: jaTr("draft with past expires", "B")},
			{Type: domain.TypeInfo, PublishedAt: &fixedNow, Translations: jaTr("published_at==now", "B")},
			{Type: domain.TypeInfo, PublishedAt: &farPast, ExpiresAt: &fixedNow, Translations: jaTr("expires_at==now", "B")},
		}
		for _, p := range seeds {
			_, err := repo.Create(ctx, p)
			require.NoError(t, err)
		}

		cases := []struct {
			name       string
			state      *string
			wantTitles []string
		}{
			{
				name:  "state=nil のとき、全件を返す",
				state: nil,
				wantTitles: []string{
					"draft", "scheduled", "published", "expired", "draft with past expires",
					"published_at==now", "expires_at==now",
				},
			},
			{
				name:       "state=Draft のとき、PublishedAt IS NULL の 2 件を返す (expires_at の値に依存しない)",
				state:      &draft,
				wantTitles: []string{"draft", "draft with past expires"},
			},
			{
				name:       "state=Scheduled のとき、PublishedAt > now の 1 件を返す (PublishedAt==now は含まない)",
				state:      &scheduled,
				wantTitles: []string{"scheduled"},
			},
			{
				name:       "state=Published のとき、公開時刻到達済みかつ未失効の行を返す (PublishedAt==now を含み、ExpiresAt==now は含まない)",
				state:      &published,
				wantTitles: []string{"published", "published_at==now"},
			},
			{
				name:       "state=Expired のとき、PublishedAt が過去かつ ExpiresAt が過去以下の行を返す (ExpiresAt==now を含む、Draft 系は含まない)",
				state:      &expired,
				wantTitles: []string{"expired", "expires_at==now"},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				items, err := repo.List(ctx, tc.state, fixedNow)
				require.NoError(t, err)

				titles := make([]string, 0, len(items))
				for _, it := range items {
					for _, tr := range it.Translations {
						titles = append(titles, tr.Title)
					}
				}
				assert.ElementsMatch(t, tc.wantTitles, titles)
			})
		}
	})
}

func TestGetPublishedDetail(t *testing.T) {
	t.Run("公開告知詳細の取得", func(t *testing.T) {
		t.Run("published_at の値に依存せず翻訳行があれば返る", func(t *testing.T) {
			// 公開期間外でもアクセス可 (published_at に依存しない)。
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			past := fixedNow.Add(-time.Hour)
			future := fixedNow.Add(time.Hour)

			mustCreate := func(p domain.CreateAnnouncementParams) int64 {
				t.Helper()
				id, err := repo.Create(ctx, p)
				require.NoError(t, err)
				return id
			}
			pastID := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &past, Translations: jaTr("past", "B-past")})
			nullID := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: nil, Translations: jaTr("null", "B-null")})
			futureID := mustCreate(domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &future, Translations: jaTr("future", "B-future")})

			cases := []struct {
				name     string
				id       int64
				wantBody string
			}{
				{
					name:     "published_at<=now のとき、body を返す",
					id:       pastID,
					wantBody: "B-past",
				},
				{
					name:     "published_at=NULL のとき、body を返す",
					id:       nullID,
					wantBody: "B-null",
				},
				{
					name:     "published_at>now のとき、body を返す",
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

		t.Run("対象が無いとき、ErrNotFound になり detail は nil", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			past := fixedNow.Add(-time.Hour)
			jaOnlyID, err := repo.Create(ctx, domain.CreateAnnouncementParams{Type: domain.TypeInfo, PublishedAt: &past, Translations: jaTr("ja only", "B")})
			require.NoError(t, err)

			cases := []struct {
				name string
				id   int64
				lang string
			}{
				{
					name: "指定 lang の翻訳行が無い (ja のみ存在する行に en で問い合わせ) とき、ErrNotFound になる",
					id:   jaOnlyID,
					lang: domain.LangEn,
				},
				{
					name: "announcement_id の行が無いとき、ErrNotFound になる",
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

func TestUpdate(t *testing.T) {
	t.Run("告知の更新", func(t *testing.T) {
		t.Run("本体属性 (type / published_at) が更新される", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()
			newTime := fixedNow.Add(time.Hour)

			id, err := repo.Create(ctx, domain.CreateAnnouncementParams{Type: domain.TypeInfo, Translations: jaTr("T", "B")})
			require.NoError(t, err)

			require.NoError(t, repo.Update(ctx, id, domain.UpdateAnnouncementParams{
				Type:        domain.TypeEvent,
				PublishedAt: &newTime,
			}))

			aw, err := repo.GetWithTranslations(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, domain.TypeEvent, aw.Announcement.Type)
			require.NotNil(t, aw.Announcement.PublishedAt)
			assert.True(t, newTime.Equal(*aw.Announcement.PublishedAt), "want=%v got=%v", newTime, *aw.Announcement.PublishedAt)
		})

		t.Run("未存在 ID のとき、port.ErrNotFound になる", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			err := repo.Update(ctx, 999999, domain.UpdateAnnouncementParams{Type: domain.TypeInfo})
			assert.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestUpsertTranslation(t *testing.T) {
	t.Run("翻訳の UPSERT", func(t *testing.T) {
		// 対象 (announcement_id, lang) の 1 行のみを変更し、他 lang 行には触れないことを確かめる。
		cases := []struct {
			name            string
			seed            []domain.TranslationInput
			upsertLang      string
			upsertTitle     string
			upsertBody      string
			wantTitleByLang map[string]string
			wantBodyByLang  map[string]string
		}{
			{
				name:        "ja のみの状態に en を追加するとき (INSERT)、ja 行は変わらない",
				seed:        jaTr("T-ja-v1", "B-ja-v1"),
				upsertLang:  domain.LangEn,
				upsertTitle: "T-en-v1",
				upsertBody:  "B-en-v1",
				wantTitleByLang: map[string]string{
					domain.LangJa: "T-ja-v1",
					domain.LangEn: "T-en-v1",
				},
				wantBodyByLang: map[string]string{
					domain.LangJa: "B-ja-v1",
					domain.LangEn: "B-en-v1",
				},
			},
			{
				name: "en を更新するとき (UPDATE)、ja 行は v1 のまま",
				seed: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T-ja-v1", Body: "B-ja-v1"},
					{Lang: domain.LangEn, Title: "T-en-v1", Body: "B-en-v1"},
				},
				upsertLang:  domain.LangEn,
				upsertTitle: "T-en-v2",
				upsertBody:  "B-en-v2",
				wantTitleByLang: map[string]string{
					domain.LangJa: "T-ja-v1",
					domain.LangEn: "T-en-v2",
				},
				wantBodyByLang: map[string]string{
					domain.LangJa: "B-ja-v1",
					domain.LangEn: "B-en-v2",
				},
			},
			{
				name: "ja を更新するとき (UPDATE)、en 行は v1 のまま",
				seed: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T-ja-v1", Body: "B-ja-v1"},
					{Lang: domain.LangEn, Title: "T-en-v1", Body: "B-en-v1"},
				},
				upsertLang:  domain.LangJa,
				upsertTitle: "T-ja-v2",
				upsertBody:  "B-ja-v2",
				wantTitleByLang: map[string]string{
					domain.LangJa: "T-ja-v2",
					domain.LangEn: "T-en-v1",
				},
				wantBodyByLang: map[string]string{
					domain.LangJa: "B-ja-v2",
					domain.LangEn: "B-en-v1",
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repo := newAnnouncementRepo(t)
				ctx := context.Background()

				id, err := repo.Create(ctx, domain.CreateAnnouncementParams{
					Type:         domain.TypeInfo,
					Translations: tc.seed,
				})
				require.NoError(t, err)

				require.NoError(t, repo.UpsertTranslation(ctx, id, tc.upsertLang, tc.upsertTitle, tc.upsertBody))

				aw, err := repo.GetWithTranslations(ctx, id)
				require.NoError(t, err)

				gotTitleByLang := make(map[string]string, len(aw.Translations))
				gotBodyByLang := make(map[string]string, len(aw.Translations))
				for _, tr := range aw.Translations {
					gotTitleByLang[tr.Lang] = tr.Title
					gotBodyByLang[tr.Lang] = tr.Body
				}
				assert.Equal(t, tc.wantTitleByLang, gotTitleByLang)
				assert.Equal(t, tc.wantBodyByLang, gotBodyByLang)
			})
		}

		t.Run("親記事が無いとき、port.ErrNotFound になる", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			err := repo.UpsertTranslation(ctx, 999999, domain.LangJa, "T", "B")
			assert.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestDelete(t *testing.T) {
	t.Run("告知の削除", func(t *testing.T) {
		t.Run("削除すると翻訳行も FK CASCADE で同時に削除される", func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			id, err := repo.Create(ctx, domain.CreateAnnouncementParams{Type: domain.TypeInfo, Translations: jaTr("T", "B")})
			require.NoError(t, err)
			require.NoError(t, repo.UpsertTranslation(ctx, id, domain.LangEn, "T", "B"))

			require.NoError(t, repo.Delete(ctx, id))

			_, err = repo.GetWithTranslations(ctx, id)
			assert.ErrorIs(t, err, port.ErrNotFound)
		})

		cases := []struct {
			name  string
			setup func(t *testing.T, repo *postgres.AnnouncementRepository, ctx context.Context) int64
		}{
			{
				name: "未存在 ID のとき、port.ErrNotFound になる",
				setup: func(_ *testing.T, _ *postgres.AnnouncementRepository, _ context.Context) int64 {
					return 999999
				},
			},
			{
				name: "一度削除済みの ID を再度削除するとき、port.ErrNotFound になる",
				setup: func(t *testing.T, repo *postgres.AnnouncementRepository, ctx context.Context) int64 {
					id, err := repo.Create(ctx, domain.CreateAnnouncementParams{Type: domain.TypeInfo, Translations: jaTr("T", "B")})
					require.NoError(t, err)
					require.NoError(t, repo.Delete(ctx, id))
					return id
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repo := newAnnouncementRepo(t)
				ctx := context.Background()
				id := tc.setup(t, repo, ctx)

				err := repo.Delete(ctx, id)
				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		}
	})
}
