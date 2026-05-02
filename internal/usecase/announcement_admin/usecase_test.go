package announcementadmin_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/usecase/announcement_admin"
)

var fixedNow = time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

func nowFixed() time.Time { return fixedNow }

// 仕様 (FEATURE_SPEC §6.4 / §6.6): Create は type と翻訳群をバリデートする。
func TestCreate_Validation(t *testing.T) {
	cases := []struct {
		name    string
		params  domain.CreateAnnouncementParams
		wantErr error
	}{
		{
			name: "未知 type",
			params: domain.CreateAnnouncementParams{
				Type: "nope",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T", Body: "B"},
				},
			},
			wantErr: announcementadmin.ErrInvalidType,
		},
		{
			name: "ja 翻訳なし",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangEn, Title: "T", Body: "B"},
				},
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name: "翻訳空 (ja 必須違反)",
			params: domain.CreateAnnouncementParams{
				Type:         "info",
				Translations: nil,
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name: "ja_title 空",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "", Body: "B"},
				},
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name: "ja_body 空",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T", Body: ""},
				},
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name: "en_title 空",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T", Body: "B"},
					{Lang: domain.LangEn, Title: "", Body: "B"},
				},
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name: "対応外 lang",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T", Body: "B"},
					{Lang: "fr", Title: "T", Body: "B"},
				},
			},
			wantErr: announcementadmin.ErrUnsupportedLang,
		},
		{
			name: "lang 重複",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "T", Body: "B"},
					{Lang: domain.LangJa, Title: "T2", Body: "B2"},
				},
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name: "ja_title 上限超過 (201 文字)",
			params: domain.CreateAnnouncementParams{
				Type: "info",
				Translations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: strings.Repeat("あ", 201), Body: "B"},
				},
			},
			wantErr: announcementadmin.ErrInvalidField,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := announcementadmin.New(&port.MockAnnouncementRepo{}, nowFixed).Create(context.Background(), tc.params)

			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.6): title は rune 数 200 以下が許容される境界。
func TestCreate_TitleBoundary(t *testing.T) {
	repo := &port.MockAnnouncementRepo{
		CreateFn: func(_ context.Context, _ domain.CreateAnnouncementParams) (int64, error) {
			return 1, nil
		},
	}
	_, err := announcementadmin.New(repo, nowFixed).Create(context.Background(), domain.CreateAnnouncementParams{
		Type: "info",
		Translations: []domain.TranslationInput{
			{Lang: domain.LangJa, Title: strings.Repeat("あ", 200), Body: "B"},
		},
	})

	require.NoError(t, err)
}

// 仕様 (FEATURE_SPEC §6.4): Create はバリデーション通過後、翻訳群をそのまま port に転送する。
func TestCreate_ForwardsTranslations(t *testing.T) {
	var got domain.CreateAnnouncementParams
	repo := &port.MockAnnouncementRepo{
		CreateFn: func(_ context.Context, p domain.CreateAnnouncementParams) (int64, error) {
			got = p
			return 7, nil
		},
	}
	id, err := announcementadmin.New(repo, nowFixed).Create(context.Background(), domain.CreateAnnouncementParams{
		Type: "info",
		Translations: []domain.TranslationInput{
			{Lang: domain.LangJa, Title: "T-ja", Body: "B-ja"},
			{Lang: domain.LangEn, Title: "T-en", Body: "B-en"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(7), id)
	assert.Equal(t, domain.TypeInfo, got.Type)
	assert.Equal(t, []domain.TranslationInput{
		{Lang: domain.LangJa, Title: "T-ja", Body: "B-ja"},
		{Lang: domain.LangEn, Title: "T-en", Body: "B-en"},
	}, got.Translations)
}

// 仕様 (FEATURE_SPEC §6.1 / §6.3): List は status クエリ値を *AnnouncementState に変換して repo に渡す。
func TestList_FilterParsing(t *testing.T) {
	draft := domain.StateDraft
	scheduled := domain.StateScheduled
	published := domain.StatePublished
	expired := domain.StateExpired

	cases := []struct {
		name      string
		filter    string
		wantState *string
	}{
		{
			name:      "空文字は全件 (nil)",
			filter:    "",
			wantState: nil,
		},
		{
			name:      "all は全件 (nil)",
			filter:    "all",
			wantState: nil,
		},
		{
			name:      "draft",
			filter:    "draft",
			wantState: &draft,
		},
		{
			name:      "scheduled",
			filter:    "scheduled",
			wantState: &scheduled,
		},
		{
			name:      "published",
			filter:    "published",
			wantState: &published,
		},
		{
			name:      "expired",
			filter:    "expired",
			wantState: &expired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotState *string
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, s *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					gotState = s
					return nil, nil
				},
			}
			_, err := announcementadmin.New(repo, nowFixed).List(context.Background(), tc.filter)

			require.NoError(t, err)
			assert.Equal(t, tc.wantState, gotState)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.1 / §6.3): 未知の status クエリ値は ErrInvalidStatusFilter を返し、repo は呼ばれない。
func TestList_UnknownFilter(t *testing.T) {
	// ListFn を未設定にすることで、呼ばれた瞬間に panic して「呼ばれていない」ことを担保する (MockAnnouncementRepo 契約)。
	repo := &port.MockAnnouncementRepo{}
	_, err := announcementadmin.New(repo, nowFixed).List(context.Background(), "invalid")

	assert.ErrorIs(t, err, announcementadmin.ErrInvalidStatusFilter)
}

// 仕様: Get も Update / Delete と同じ repo エラーマッピング規約。
func TestGet(t *testing.T) {
	dbErr := errors.New("db lost")
	okResult := &domain.AnnouncementWithTranslations{
		Announcement: domain.Announcement{AnnouncementID: 1, Type: domain.TypeInfo},
	}

	cases := []struct {
		name       string
		repoResult *domain.AnnouncementWithTranslations
		repoErr    error
		wantErr    error
	}{
		{
			name:       "成功 (nil → nil)",
			repoResult: okResult,
			repoErr:    nil,
			wantErr:    nil,
		},
		{
			name:       "port.ErrNotFound は usecase ErrNotFound にマップ",
			repoResult: nil,
			repoErr:    port.ErrNotFound,
			wantErr:    announcementadmin.ErrNotFound,
		},
		{
			name:       "その他のエラーは %w で透過 (元 err が chain 内にある)",
			repoResult: nil,
			repoErr:    dbErr,
			wantErr:    dbErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return tc.repoResult, tc.repoErr
				},
			}
			_, err := announcementadmin.New(repo, nowFixed).Get(context.Background(), 1)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.3): DeriveState は (PublishedAt, ExpiresAt) と now から state を一意に導出する。
// 判定順序が排他性を担保している。特に PublishedAt IS NULL の record は ExpiresAt の値に依存せず Draft。
func TestDeriveState(t *testing.T) {
	past := fixedNow.Add(-time.Hour)
	future := fixedNow.Add(time.Hour)

	cases := []struct {
		name        string
		publishedAt *time.Time
		expiresAt   *time.Time
		want        string
	}{
		{
			name:        "PublishedAt=NULL → Draft (ExpiresAt 未設定)",
			publishedAt: nil,
			expiresAt:   nil,
			want:        domain.StateDraft,
		},
		{
			name:        "PublishedAt=NULL → Draft (ExpiresAt 過去でも Expired にならない)",
			publishedAt: nil,
			expiresAt:   &past,
			want:        domain.StateDraft,
		},
		{
			name:        "PublishedAt>now → Scheduled",
			publishedAt: &future,
			expiresAt:   nil,
			want:        domain.StateScheduled,
		},
		{
			name:        "PublishedAt<=now ∧ ExpiresAt=NULL → Published",
			publishedAt: &past,
			expiresAt:   nil,
			want:        domain.StatePublished,
		},
		{
			name:        "PublishedAt<=now ∧ ExpiresAt>now → Published",
			publishedAt: &past,
			expiresAt:   &future,
			want:        domain.StatePublished,
		},
		{
			name:        "PublishedAt<=now ∧ ExpiresAt<=now → Expired",
			publishedAt: &past,
			expiresAt:   &past,
			want:        domain.StateExpired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := announcementadmin.DeriveState(domain.Announcement{
				PublishedAt: tc.publishedAt,
				ExpiresAt:   tc.expiresAt,
			}, fixedNow)
			assert.Equal(t, tc.want, got)
		})
	}
}

// 仕様: Update は repo のエラーをセンチネルにマップ (port.ErrNotFound → usecase.ErrNotFound、それ以外は %w で透過)。
func TestUpdate(t *testing.T) {
	dbErr := errors.New("db")

	cases := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{
			name:    "成功 (nil → nil)",
			repoErr: nil,
			wantErr: nil,
		},
		{
			name:    "port.ErrNotFound は usecase ErrNotFound にマップ",
			repoErr: port.ErrNotFound,
			wantErr: announcementadmin.ErrNotFound,
		},
		{
			name:    "その他のエラーは %w で透過 (元 err が chain 内にある)",
			repoErr: dbErr,
			wantErr: dbErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				UpdateFn: func(_ context.Context, _ int64, _ domain.UpdateAnnouncementParams) error {
					return tc.repoErr
				},
			}
			err := announcementadmin.New(repo, nowFixed).Update(context.Background(), 1, domain.UpdateAnnouncementParams{Type: "info"})
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様: Delete も Update と同じ repo エラーマッピング規約。
func TestDelete(t *testing.T) {
	dbErr := errors.New("db")

	cases := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{
			name:    "成功 (nil → nil)",
			repoErr: nil,
			wantErr: nil,
		},
		{
			name:    "port.ErrNotFound は usecase ErrNotFound にマップ",
			repoErr: port.ErrNotFound,
			wantErr: announcementadmin.ErrNotFound,
		},
		{
			name:    "その他のエラーは %w で透過 (元 err が chain 内にある)",
			repoErr: dbErr,
			wantErr: dbErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				DeleteFn: func(_ context.Context, _ int64) error {
					return tc.repoErr
				},
			}
			err := announcementadmin.New(repo, nowFixed).Delete(context.Background(), 1)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.5 / §6.6): UpsertTranslation は lang / title / body をバリデートする。
func TestUpsertTranslation_Validation(t *testing.T) {
	cases := []struct {
		name    string
		lang    string
		title   string
		body    string
		wantErr error
	}{
		{
			name:    "対応外 lang",
			lang:    "fr",
			title:   "T",
			body:    "B",
			wantErr: announcementadmin.ErrUnsupportedLang,
		},
		{
			name:    "title 空",
			lang:    domain.LangJa,
			title:   "",
			body:    "B",
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name:    "body 空",
			lang:    domain.LangJa,
			title:   "T",
			body:    "",
			wantErr: announcementadmin.ErrInvalidField,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := announcementadmin.New(&port.MockAnnouncementRepo{}, nowFixed).UpsertTranslation(context.Background(), 1, tc.lang, tc.title, tc.body)

			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様: UpsertTranslation は repo の ErrNotFound を usecase の ErrNotFound にマップする。
func TestUpsertTranslation_NotFound(t *testing.T) {
	repo := &port.MockAnnouncementRepo{
		UpsertTranslationFn: func(_ context.Context, _ int64, _ string, _ string, _ string) error {
			return port.ErrNotFound
		},
	}
	err := announcementadmin.New(repo, nowFixed).UpsertTranslation(context.Background(), 999, domain.LangJa, "T", "B")
	assert.ErrorIs(t, err, announcementadmin.ErrNotFound)
}
