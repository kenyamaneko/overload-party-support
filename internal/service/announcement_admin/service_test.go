package announcementadmin_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/service/announcement_admin"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

var fixedNow = time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

func nowFixed() time.Time { return fixedNow }

// 仕様 (FEATURE_SPEC §6.1 / §6.3): List は既知の status クエリ値を port.StatusFilter に変換して repo に渡す。
// 空文字 / "all" は全件フィルタ、それ以外の既知値は対応する StatusFilter に対応する。
func TestList_仕様_フィルタパース(t *testing.T) {
	cases := []struct {
		name       string
		filter     string
		wantFilter port.StatusFilter
	}{
		{
			name:       "空文字は all",
			filter:     "",
			wantFilter: port.StatusFilterAll,
		},
		{
			name:       "all",
			filter:     "all",
			wantFilter: port.StatusFilterAll,
		},
		{
			name:       "draft",
			filter:     "draft",
			wantFilter: port.StatusFilterDraft,
		},
		{
			name:       "scheduled",
			filter:     "scheduled",
			wantFilter: port.StatusFilterScheduled,
		},
		{
			name:       "published",
			filter:     "published",
			wantFilter: port.StatusFilterPublished,
		},
		{
			name:       "expired",
			filter:     "expired",
			wantFilter: port.StatusFilterExpired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotFilter port.StatusFilter
			repo := &port.MockAnnouncementRepo{
				ListAllFn: func(_ context.Context, f port.StatusFilter, _ time.Time) ([]apisupport.AnnouncementWithTranslations, error) {
					gotFilter = f
					return nil, nil
				},
			}
			_, err := announcementadmin.New(repo, nowFixed).List(context.Background(), tc.filter)

			require.NoError(t, err)
			assert.Equal(t, tc.wantFilter, gotFilter)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.1 / §6.3): 未知の status クエリ値は ErrInvalidStatusFilter を返し、repo は呼ばれない。
func TestList_仕様_未知値はErrInvalidStatusFilter(t *testing.T) {
	// ListAllFn を未設定にすることで、呼ばれた瞬間に panic して「呼ばれていない」ことを担保する (MockAnnouncementRepo 契約)。
	repo := &port.MockAnnouncementRepo{}
	_, err := announcementadmin.New(repo, nowFixed).List(context.Background(), "invalid")

	assert.ErrorIs(t, err, announcementadmin.ErrInvalidStatusFilter)
}

// 仕様 (FEATURE_SPEC §6.4 / §6.6): Create は type と翻訳群をバリデートする。
//   - ja 翻訳が少なくとも 1 件必須
//   - 各翻訳は lang が対応言語 / title 非空 / title <= 200 / body 非空
//   - lang の重複は不可
func TestCreate_仕様_typeと翻訳群のバリデーション(t *testing.T) {
	cases := []struct {
		name     string
		params   announcementadmin.CreateParams
		wantErr  error
		wantCall bool
	}{
		{
			name: "ja のみ 正常",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: "B"},
				},
			},
			wantErr:  nil,
			wantCall: true,
		},
		{
			name: "ja + en 正常",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: "B"},
					{Lang: apisupport.LangEn, Title: "T-en", Body: "B-en"},
				},
			},
			wantErr:  nil,
			wantCall: true,
		},
		{
			name: "未知 type",
			params: announcementadmin.CreateParams{
				Type: "nope",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: "B"},
				},
			},
			wantErr:  announcementadmin.ErrInvalidType,
			wantCall: false,
		},
		{
			name: "ja 翻訳なし",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangEn, Title: "T", Body: "B"},
				},
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "翻訳空 (ja 必須違反)",
			params: announcementadmin.CreateParams{
				Type:         "info",
				Translations: nil,
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "ja_title 空",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "", Body: "B"},
				},
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "ja_body 空",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: ""},
				},
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "en_title 空",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: "B"},
					{Lang: apisupport.LangEn, Title: "", Body: "B"},
				},
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "対応外 lang",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: "B"},
					{Lang: "fr", Title: "T", Body: "B"},
				},
			},
			wantErr:  announcementadmin.ErrUnsupportedLang,
			wantCall: false,
		},
		{
			name: "lang 重複",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: "T", Body: "B"},
					{Lang: apisupport.LangJa, Title: "T2", Body: "B2"},
				},
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "ja_title 上限超過 (201 文字)",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: strings.Repeat("あ", 201), Body: "B"},
				},
			},
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name: "ja_title 上限ちょうど (200 文字)",
			params: announcementadmin.CreateParams{
				Type: "info",
				Translations: []announcementadmin.TranslationInput{
					{Lang: apisupport.LangJa, Title: strings.Repeat("あ", 200), Body: "B"},
				},
			},
			wantErr:  nil,
			wantCall: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			repo := &port.MockAnnouncementRepo{
				CreateFn: func(_ context.Context, _ port.CreateAnnouncementParams) (int64, error) {
					called = true
					return 42, nil
				},
			}
			_, err := announcementadmin.New(repo, nowFixed).Create(context.Background(), tc.params)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCall, called)
		})
	}
}

// 仕様 (FEATURE_SPEC §6.4): Create はバリデーション通過後、翻訳群をそのまま port に転送する。
func TestCreate_仕様_翻訳群をportに転送(t *testing.T) {
	var got port.CreateAnnouncementParams
	repo := &port.MockAnnouncementRepo{
		CreateFn: func(_ context.Context, p port.CreateAnnouncementParams) (int64, error) {
			got = p
			return 7, nil
		},
	}
	id, err := announcementadmin.New(repo, nowFixed).Create(context.Background(), announcementadmin.CreateParams{
		Type: "info",
		Translations: []announcementadmin.TranslationInput{
			{Lang: apisupport.LangJa, Title: "T-ja", Body: "B-ja"},
			{Lang: apisupport.LangEn, Title: "T-en", Body: "B-en"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(7), id)
	assert.Equal(t, apisupport.TypeInfo, got.Type)
	assert.Equal(t, []port.TranslationInput{
		{Lang: apisupport.LangJa, Title: "T-ja", Body: "B-ja"},
		{Lang: apisupport.LangEn, Title: "T-en", Body: "B-en"},
	}, got.Translations)
}

// 仕様: Update は repo のエラーを以下にマップする。
//   - port.ErrNotFound → service の ErrNotFound (sentinel 入れ替え)
//   - その他のエラー → %w でラップして透過 (errors.Is で元 err に到達可能)
//   - nil → nil
//
// errors.Is は wrap チェーンを辿って一致を見るので、透過ケースでも repoErr を target にして検出できる。
func TestUpdate_仕様_repoエラーマッピング(t *testing.T) {
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
			name:    "port.ErrNotFound は service ErrNotFound にマップ",
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
				UpdateFn: func(_ context.Context, _ int64, _ port.UpdateAnnouncementParams) error {
					return tc.repoErr
				},
			}
			err := announcementadmin.New(repo, nowFixed).Update(context.Background(), 1, announcementadmin.UpdateParams{Type: "info"})
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様: Delete も Update と同じ repo エラーマッピング規約。
func TestDelete_仕様_repoエラーマッピング(t *testing.T) {
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
			name:    "port.ErrNotFound は service ErrNotFound にマップ",
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
func TestUpsertTranslation_仕様_バリデーション(t *testing.T) {
	cases := []struct {
		name     string
		lang     string
		title    string
		body     string
		wantErr  error
		wantCall bool
	}{
		{
			name:     "ja 正常",
			lang:     apisupport.LangJa,
			title:    "T",
			body:     "B",
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "en 正常",
			lang:     apisupport.LangEn,
			title:    "T",
			body:     "B",
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "対応外 lang",
			lang:     "fr",
			title:    "T",
			body:     "B",
			wantErr:  announcementadmin.ErrUnsupportedLang,
			wantCall: false,
		},
		{
			name:     "title 空",
			lang:     apisupport.LangJa,
			title:    "",
			body:     "B",
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
		{
			name:     "body 空",
			lang:     apisupport.LangJa,
			title:    "T",
			body:     "",
			wantErr:  announcementadmin.ErrInvalidField,
			wantCall: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			repo := &port.MockAnnouncementRepo{
				UpsertTranslationFn: func(_ context.Context, _ int64, _ string, _ string, _ string) error {
					called = true
					return nil
				},
			}
			err := announcementadmin.New(repo, nowFixed).UpsertTranslation(context.Background(), 1, tc.lang, tc.title, tc.body)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCall, called)
		})
	}
}

// 仕様: Get も Update / Delete と同じ repo エラーマッピング規約。
// 成功ケースは repo が返した値をそのまま service が返す (mock は非 nil の結果を返す)。
func TestGet_仕様_repoエラーマッピング(t *testing.T) {
	dbErr := errors.New("db lost")
	okResult := &apisupport.AnnouncementWithTranslations{
		Announcement: apisupport.Announcement{AnnouncementID: 1, Type: apisupport.TypeInfo},
	}

	cases := []struct {
		name       string
		repoResult *apisupport.AnnouncementWithTranslations
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
			name:       "port.ErrNotFound は service ErrNotFound にマップ",
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
				GetFn: func(_ context.Context, _ int64) (*apisupport.AnnouncementWithTranslations, error) {
					return tc.repoResult, tc.repoErr
				},
			}
			_, err := announcementadmin.New(repo, nowFixed).Get(context.Background(), 1)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様: UpsertTranslation は repo の ErrNotFound を service の ErrNotFound にマップする。
func TestUpsertTranslation_仕様_NotFoundマップ(t *testing.T) {
	repo := &port.MockAnnouncementRepo{
		UpsertTranslationFn: func(_ context.Context, _ int64, _ string, _ string, _ string) error {
			return port.ErrNotFound
		},
	}
	err := announcementadmin.New(repo, nowFixed).UpsertTranslation(context.Background(), 999, apisupport.LangJa, "T", "B")
	assert.ErrorIs(t, err, announcementadmin.ErrNotFound)
}
