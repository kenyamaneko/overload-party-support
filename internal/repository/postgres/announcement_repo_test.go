package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres/postgrestest"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
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
func jaTr(title, body string) []port.TranslationInput {
	return []port.TranslationInput{{Lang: apisupport.LangJa, Title: title, Body: body}}
}

// 仕様 (FEATURE_SPEC §4.1): ListPublished は以下すべてを満たす行のみを返す。
//   - published_at IS NOT NULL
//   - published_at <= now
//   - expires_at IS NULL OR expires_at > now
//   - 指定 lang の翻訳行が存在する
func TestListPublished_仕様_公開対象のみ返す(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	past := fixedNow.Add(-24 * time.Hour)
	future := fixedNow.Add(24 * time.Hour)
	farPast := fixedNow.Add(-48 * time.Hour)

	mustCreate := func(t *testing.T, p port.CreateAnnouncementParams) int64 {
		t.Helper()
		id, err := repo.Create(ctx, p)
		require.NoError(t, err)
		return id
	}

	// published_at <= now かつ expires_at IS NULL。ja + en 翻訳あり → ja / en どちらでも返る
	mustCreate(t, port.CreateAnnouncementParams{
		Type:        apisupport.TypeInfo,
		PublishedAt: &past,
		Translations: []port.TranslationInput{
			{Lang: apisupport.LangJa, Title: "published_at<=now (ja+en)", Body: "B"},
			{Lang: apisupport.LangEn, Title: "published_at<=now (ja+en) [en]", Body: "B-en"},
		},
	})

	// published_at IS NULL → 公開対象外
	mustCreate(t, port.CreateAnnouncementParams{
		Type:         apisupport.TypeInfo,
		PublishedAt:  nil,
		Translations: jaTr("published_at=NULL", "B"),
	})

	// published_at > now → 公開対象外 (未到達)
	mustCreate(t, port.CreateAnnouncementParams{
		Type:         apisupport.TypeInfo,
		PublishedAt:  &future,
		Translations: jaTr("published_at>now", "B"),
	})

	// expires_at <= now → 公開対象外
	mustCreate(t, port.CreateAnnouncementParams{
		Type:         apisupport.TypeEvent,
		PublishedAt:  &farPast,
		ExpiresAt:    &past,
		Translations: jaTr("expires_at<=now", "B"),
	})

	// published_at <= now だが翻訳は ja のみ → en 問い合わせでは返らない
	mustCreate(t, port.CreateAnnouncementParams{
		Type:         apisupport.TypeInfo,
		PublishedAt:  &past,
		Translations: jaTr("published_at<=now (ja only)", "B"),
	})

	cases := []struct {
		name        string
		lang        string
		wantCount   int
		wantInclude string // 返ってきた結果に含まれるべき代表タイトル
	}{
		{
			name:        "ja: 公開条件を満たす 2 件 (ja+en 行 / ja only 行)",
			lang:        apisupport.LangJa,
			wantCount:   2,
			wantInclude: "published_at<=now (ja+en)",
		},
		{
			name:        "en: 公開条件を満たし en 翻訳もある 1 件のみ",
			lang:        apisupport.LangEn,
			wantCount:   1,
			wantInclude: "published_at<=now (ja+en) [en]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := repo.ListPublished(ctx, tc.lang, fixedNow)
			require.NoError(t, err)
			assert.Len(t, items, tc.wantCount)

			titles := make([]string, 0, len(items))
			for _, it := range items {
				titles = append(titles, it.Title)
			}
			assert.Contains(t, titles, tc.wantInclude)
		})
	}
}

// 仕様 (FEATURE_SPEC §4.1): 空 DB に対する ListPublished はエラーなく空 slice を返す。
// nil ではなく長さ 0 の slice を返すこと (呼び出し側が range できる契約)。
func TestListPublished_仕様_空DB(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	items, err := repo.ListPublished(ctx, apisupport.LangJa, fixedNow)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// 仕様 (FEATURE_SPEC §4.1): 並び順は published_at DESC, announcement_id DESC。
func TestListPublished_仕様_並び順(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	t1 := fixedNow.Add(-3 * time.Hour)
	t2 := fixedNow.Add(-2 * time.Hour)
	t3 := fixedNow.Add(-1 * time.Hour)

	// t1 で 2 件、t2 で 1 件、t3 で 1 件
	id1a, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &t1, Translations: jaTr("1a", "B")})
	require.NoError(t, err)
	id1b, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &t1, Translations: jaTr("1b", "B")})
	require.NoError(t, err)
	id2, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &t2, Translations: jaTr("2", "B")})
	require.NoError(t, err)
	id3, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &t3, Translations: jaTr("3", "B")})
	require.NoError(t, err)

	items, err := repo.ListPublished(ctx, apisupport.LangJa, fixedNow)
	require.NoError(t, err)
	require.Len(t, items, 4)

	// t3 が先頭、その後 t2、最後に t1 の 2 件 (id 降順)
	assert.Equal(t, id3, items[0].AnnouncementID)
	assert.Equal(t, id2, items[1].AnnouncementID)
	assert.Equal(t, id1b, items[2].AnnouncementID)
	assert.Equal(t, id1a, items[3].AnnouncementID)
}

// 仕様 (FEATURE_SPEC §5): GetPublishedDetail は published_at の値 (過去 / NULL / 未来) に依存せず、
// 翻訳行が存在する限り返す (公開期間外でもアクセス可)。
func TestGetPublishedDetail_仕様_publishedAtに依存せず返す(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	past := fixedNow.Add(-time.Hour)
	future := fixedNow.Add(time.Hour)

	pastID, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &past, Translations: jaTr("past", "B-past")})
	require.NoError(t, err)
	nullID, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: nil, Translations: jaTr("null", "B-null")})
	require.NoError(t, err)
	futureID, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &future, Translations: jaTr("future", "B-future")})
	require.NoError(t, err)

	cases := []struct {
		name     string
		id       int64
		wantBody string
	}{
		{
			name:     "published_at<=now",
			id:       pastID,
			wantBody: "B-past",
		},
		{
			name:     "published_at=NULL",
			id:       nullID,
			wantBody: "B-null",
		},
		{
			name:     "published_at>now",
			id:       futureID,
			wantBody: "B-future",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detail, err := repo.GetPublishedDetail(ctx, tc.id, apisupport.LangJa)
			require.NoError(t, err)
			require.NotNil(t, detail)
			assert.Equal(t, tc.wantBody, detail.Body)
		})
	}
}

// 仕様 (FEATURE_SPEC §5): GetPublishedDetail は以下の場合に port.ErrNotFound を返し、detail は nil。
//   - 指定 lang の翻訳行が存在しない
//   - announcement_id の行自体が存在しない
func TestGetPublishedDetail_仕様_ErrNotFound(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	past := fixedNow.Add(-time.Hour)
	jaOnlyID, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &past, Translations: jaTr("ja only", "B")})
	require.NoError(t, err)

	cases := []struct {
		name string
		id   int64
		lang string
	}{
		{
			name: "指定 lang の翻訳行が無い (ja のみ存在する行に en で問い合わせ)",
			id:   jaOnlyID,
			lang: apisupport.LangEn,
		},
		{
			name: "announcement_id の行が無い",
			id:   999999,
			lang: apisupport.LangJa,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detail, err := repo.GetPublishedDetail(ctx, tc.id, tc.lang)
			assert.ErrorIs(t, err, port.ErrNotFound)
			assert.Nil(t, detail)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.4): Create は本体 + 受け取った翻訳群を同一 tx で INSERT する。
// repo はロジックを持たず、渡された翻訳をそのまま永続化する (0 件 / ja のみ / ja+en いずれも許容)。
// ja 必須などの業務制約は service 層の責務であり、repo 層のテストでは扱わない。
func TestCreate_仕様_本体と翻訳群を同時作成(t *testing.T) {
	past := fixedNow.Add(-time.Hour)

	cases := []struct {
		name         string
		params       port.CreateAnnouncementParams
		wantTrByLang map[string]string // lang -> title
	}{
		{
			name: "翻訳 0 件でも本体行は作られる (repo は素通す)",
			params: port.CreateAnnouncementParams{
				Type:         apisupport.TypeInfo,
				PublishedAt:  &past,
				Translations: nil,
			},
			wantTrByLang: map[string]string{},
		},
		{
			name: "ja のみ",
			params: port.CreateAnnouncementParams{
				Type:         apisupport.TypeInfo,
				PublishedAt:  &past,
				Translations: jaTr("T", "B"),
			},
			wantTrByLang: map[string]string{apisupport.LangJa: "T"},
		},
		{
			name: "ja + en 同時 INSERT",
			params: port.CreateAnnouncementParams{
				Type:        apisupport.TypeInfo,
				PublishedAt: &past,
				Translations: []port.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T-ja", Body: "B-ja"},
					{Lang: apisupport.LangEn, Title: "T-en", Body: "B-en"},
				},
			},
			wantTrByLang: map[string]string{apisupport.LangJa: "T-ja", apisupport.LangEn: "T-en"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			id, err := repo.Create(ctx, tc.params)
			require.NoError(t, err)
			assert.Greater(t, id, int64(0))

			aw, err := repo.Get(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, aw)
			assert.Equal(t, tc.params.Type, aw.Announcement.Type)

			gotByLang := make(map[string]string, len(aw.Translations))
			for _, tr := range aw.Translations {
				gotByLang[tr.Lang] = tr.Title
			}
			assert.Equal(t, tc.wantTrByLang, gotByLang)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.4): Update は既存 ID の本体属性 (type / published_at) を更新する。
func TestUpdate_仕様_本体属性を更新(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()
	newTime := fixedNow.Add(time.Hour)

	id, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, Translations: jaTr("T", "B")})
	require.NoError(t, err)

	require.NoError(t, repo.Update(ctx, id, port.UpdateAnnouncementParams{
		Type:        apisupport.TypeEvent,
		PublishedAt: &newTime,
	}))

	aw, err := repo.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, apisupport.TypeEvent, aw.Announcement.Type)
	require.NotNil(t, aw.Announcement.PublishedAt)
	assert.True(t, newTime.Equal(*aw.Announcement.PublishedAt), "want=%v got=%v", newTime, *aw.Announcement.PublishedAt)
}

// 仕様 (FEATURE_SPEC §6.4): Update は未存在 ID に対して port.ErrNotFound を返す。
func TestUpdate_仕様_未存在IDはErrNotFound(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	err := repo.Update(ctx, 999999, port.UpdateAnnouncementParams{Type: apisupport.TypeInfo})
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// 仕様 (FEATURE_SPEC §6.4): Delete は翻訳を持つ既存記事を FK CASCADE で同時削除し、以後 Get で ErrNotFound になる。
func TestDelete_仕様_既存IDはCASCADE削除され再取得できない(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, Translations: jaTr("T", "B")})
	require.NoError(t, err)
	require.NoError(t, repo.UpsertTranslation(ctx, id, apisupport.LangEn, "T", "B"))

	require.NoError(t, repo.Delete(ctx, id))

	_, err = repo.Get(ctx, id)
	assert.ErrorIs(t, err, port.ErrNotFound, "Delete 後に Get で取得できないこと")
}

// 仕様 (FEATURE_SPEC §6.4): Delete は削除対象行が存在しない場合 port.ErrNotFound を返す。
// 未存在 ID も二度目の Delete も同じ振る舞い。
func TestDelete_仕様_ErrNotFound(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, repo *postgres.AnnouncementRepository, ctx context.Context) int64
	}{
		{
			name: "未存在 ID",
			setup: func(_ *testing.T, _ *postgres.AnnouncementRepository, _ context.Context) int64 {
				return 999999
			},
		},
		{
			name: "一度削除済みの ID を再度 Delete",
			setup: func(t *testing.T, repo *postgres.AnnouncementRepository, ctx context.Context) int64 {
				id, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, Translations: jaTr("T", "B")})
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
}

// 仕様 (FEATURE_SPEC §6.5): UpsertTranslation は対象 (announcement_id, lang) の 1 行のみを変更し、他 lang 行には一切触れない。
// INSERT (新規 lang 追加) と UPDATE (既存 lang 差し替え) の双方で、既存他 lang 行が維持されることを確認する。
func TestUpsertTranslation_仕様_他lang行に影響しない(t *testing.T) {
	cases := []struct {
		name            string
		seed            []port.TranslationInput
		upsertLang      string
		upsertTitle     string
		upsertBody      string
		wantTitleByLang map[string]string
		wantBodyByLang  map[string]string
	}{
		{
			name:        "INSERT: ja のみの状態に en を追加しても ja は変わらない",
			seed:        jaTr("T-ja-v1", "B-ja-v1"),
			upsertLang:  apisupport.LangEn,
			upsertTitle: "T-en-v1",
			upsertBody:  "B-en-v1",
			wantTitleByLang: map[string]string{
				apisupport.LangJa: "T-ja-v1",
				apisupport.LangEn: "T-en-v1",
			},
			wantBodyByLang: map[string]string{
				apisupport.LangJa: "B-ja-v1",
				apisupport.LangEn: "B-en-v1",
			},
		},
		{
			name: "UPDATE: en 更新時、ja 行は v1 のまま",
			seed: []port.TranslationInput{
				{Lang: apisupport.LangJa, Title: "T-ja-v1", Body: "B-ja-v1"},
				{Lang: apisupport.LangEn, Title: "T-en-v1", Body: "B-en-v1"},
			},
			upsertLang:  apisupport.LangEn,
			upsertTitle: "T-en-v2",
			upsertBody:  "B-en-v2",
			wantTitleByLang: map[string]string{
				apisupport.LangJa: "T-ja-v1",
				apisupport.LangEn: "T-en-v2",
			},
			wantBodyByLang: map[string]string{
				apisupport.LangJa: "B-ja-v1",
				apisupport.LangEn: "B-en-v2",
			},
		},
		{
			name: "UPDATE: ja 更新時、en 行は v1 のまま",
			seed: []port.TranslationInput{
				{Lang: apisupport.LangJa, Title: "T-ja-v1", Body: "B-ja-v1"},
				{Lang: apisupport.LangEn, Title: "T-en-v1", Body: "B-en-v1"},
			},
			upsertLang:  apisupport.LangJa,
			upsertTitle: "T-ja-v2",
			upsertBody:  "B-ja-v2",
			wantTitleByLang: map[string]string{
				apisupport.LangJa: "T-ja-v2",
				apisupport.LangEn: "T-en-v1",
			},
			wantBodyByLang: map[string]string{
				apisupport.LangJa: "B-ja-v2",
				apisupport.LangEn: "B-en-v1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAnnouncementRepo(t)
			ctx := context.Background()

			id, err := repo.Create(ctx, port.CreateAnnouncementParams{
				Type:         apisupport.TypeInfo,
				Translations: tc.seed,
			})
			require.NoError(t, err)

			require.NoError(t, repo.UpsertTranslation(ctx, id, tc.upsertLang, tc.upsertTitle, tc.upsertBody))

			aw, err := repo.Get(ctx, id)
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
}

// 仕様 (FEATURE_SPEC §6.5): UpsertTranslation は親記事が存在しなければ port.ErrNotFound。
func TestUpsertTranslation_仕様_親記事なしはErrNotFound(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	err := repo.UpsertTranslation(ctx, 999999, apisupport.LangJa, "T", "B")
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// 仕様 (FEATURE_SPEC §6.3): ListAll は status フィルタ (draft/scheduled/published/expired/all) を反映する。
func TestListAll_仕様_フィルタ(t *testing.T) {
	repo := newAnnouncementRepo(t)
	ctx := context.Background()

	past := fixedNow.Add(-time.Hour)
	future := fixedNow.Add(time.Hour)
	farPast := fixedNow.Add(-48 * time.Hour)

	_, err := repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: nil, Translations: jaTr("draft", "B")})
	require.NoError(t, err)
	_, err = repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &future, Translations: jaTr("scheduled", "B")})
	require.NoError(t, err)
	_, err = repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeInfo, PublishedAt: &past, Translations: jaTr("published", "B")})
	require.NoError(t, err)
	_, err = repo.Create(ctx, port.CreateAnnouncementParams{Type: apisupport.TypeEvent, PublishedAt: &farPast, ExpiresAt: &past, Translations: jaTr("expired", "B")})
	require.NoError(t, err)

	cases := []struct {
		name      string
		filter    port.StatusFilter
		wantCount int
	}{
		{
			name:      "all は全件",
			filter:    port.StatusFilterAll,
			wantCount: 4,
		},
		{
			name:      "draft は 1 件",
			filter:    port.StatusFilterDraft,
			wantCount: 1,
		},
		{
			name:      "scheduled は 1 件",
			filter:    port.StatusFilterScheduled,
			wantCount: 1,
		},
		{
			name:      "published は 1 件",
			filter:    port.StatusFilterPublished,
			wantCount: 1,
		},
		{
			name:      "expired は 1 件",
			filter:    port.StatusFilterExpired,
			wantCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := repo.ListAll(ctx, tc.filter, fixedNow)
			require.NoError(t, err)
			assert.Len(t, items, tc.wantCount)
		})
	}
}
