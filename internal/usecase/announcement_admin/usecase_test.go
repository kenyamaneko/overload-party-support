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

func TestCreate(t *testing.T) {
	t.Run("告知の作成", func(t *testing.T) {
		validationCases := []struct {
			name    string
			params  domain.CreateAnnouncementParams
			wantErr error
		}{
			{
				name: "type が未知のとき、ErrInvalidType になる",
				params: domain.CreateAnnouncementParams{
					Type: "nope",
					Translations: []domain.TranslationInput{
						{Lang: domain.LangJa, Title: "T", Body: "B"},
					},
				},
				wantErr: announcementadmin.ErrInvalidType,
			},
			{
				name: "ja 翻訳が無いとき、ErrInvalidField になる",
				params: domain.CreateAnnouncementParams{
					Type: "info",
					Translations: []domain.TranslationInput{
						{Lang: domain.LangEn, Title: "T", Body: "B"},
					},
				},
				wantErr: announcementadmin.ErrInvalidField,
			},
			{
				name: "翻訳が空 (ja 必須違反) のとき、ErrInvalidField になる",
				params: domain.CreateAnnouncementParams{
					Type:         "info",
					Translations: nil,
				},
				wantErr: announcementadmin.ErrInvalidField,
			},
			{
				name: "ja_title が空のとき、ErrInvalidField になる",
				params: domain.CreateAnnouncementParams{
					Type: "info",
					Translations: []domain.TranslationInput{
						{Lang: domain.LangJa, Title: "", Body: "B"},
					},
				},
				wantErr: announcementadmin.ErrInvalidField,
			},
			{
				name: "ja_body が空のとき、ErrInvalidField になる",
				params: domain.CreateAnnouncementParams{
					Type: "info",
					Translations: []domain.TranslationInput{
						{Lang: domain.LangJa, Title: "T", Body: ""},
					},
				},
				wantErr: announcementadmin.ErrInvalidField,
			},
			{
				name: "en_title が空のとき、ErrInvalidField になる",
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
				name: "対応外 lang のとき、ErrUnsupportedLang になる",
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
				name: "lang が重複するとき、ErrInvalidField になる",
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
				name: "ja_title が上限超過 (201 文字) のとき、ErrInvalidField になる",
				params: domain.CreateAnnouncementParams{
					Type: "info",
					Translations: []domain.TranslationInput{
						{Lang: domain.LangJa, Title: strings.Repeat("あ", 201), Body: "B"},
					},
				},
				wantErr: announcementadmin.ErrInvalidField,
			},
		}

		for _, tc := range validationCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := announcementadmin.New(&port.MockAnnouncementRepo{}, nowFixed).Create(context.Background(), tc.params)

				assert.ErrorIs(t, err, tc.wantErr)
			})
		}

		t.Run("ja_title が上限ちょうど (200 文字) のとき、作成できる", func(t *testing.T) {
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
		})

		t.Run("作成すると、指定した種別と翻訳がそのまま保存される", func(t *testing.T) {
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
		})
	})
}

func TestList(t *testing.T) {
	t.Run("status フィルタの解析", func(t *testing.T) {
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
				name:      "空文字のとき、nil (全件) を repo に渡す",
				filter:    "",
				wantState: nil,
			},
			{
				name:      "all のとき、nil (全件) を repo に渡す",
				filter:    "all",
				wantState: nil,
			},
			{
				name:      "draft のとき、StateDraft を repo に渡す",
				filter:    "draft",
				wantState: &draft,
			},
			{
				name:      "scheduled のとき、StateScheduled を repo に渡す",
				filter:    "scheduled",
				wantState: &scheduled,
			},
			{
				name:      "published のとき、StatePublished を repo に渡す",
				filter:    "published",
				wantState: &published,
			},
			{
				name:      "expired のとき、StateExpired を repo に渡す",
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

		t.Run("未知の status のとき、ErrInvalidStatusFilter になる", func(t *testing.T) {
			// ListFn を未設定にすることで、絞り込みに到達する前に弾かれることを担保する。
			repo := &port.MockAnnouncementRepo{}
			_, err := announcementadmin.New(repo, nowFixed).List(context.Background(), "invalid")

			assert.ErrorIs(t, err, announcementadmin.ErrInvalidStatusFilter)
		})
	})
}

func TestGet(t *testing.T) {
	t.Run("告知の取得", func(t *testing.T) {
		t.Run("告知が存在するとき、その内容が返る", func(t *testing.T) {
			okResult := &domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{AnnouncementID: 1, Type: domain.TypeInfo},
			}
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return okResult, nil
				},
			}

			got, err := announcementadmin.New(repo, nowFixed).Get(context.Background(), 1)

			require.NoError(t, err)
			assert.Equal(t, okResult, got)
		})

		dbErr := errors.New("db lost")
		errorCases := []struct {
			name    string
			repoErr error
			wantErr error
		}{
			{
				name:    "告知が見つからないとき、ErrNotFound になる",
				repoErr: port.ErrNotFound,
				wantErr: announcementadmin.ErrNotFound,
			},
			{
				name:    "想定外のエラーが起きたとき、そのエラーがそのまま返る",
				repoErr: dbErr,
				wantErr: dbErr,
			},
		}

		for _, tc := range errorCases {
			t.Run(tc.name, func(t *testing.T) {
				repo := &port.MockAnnouncementRepo{
					GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
						return nil, tc.repoErr
					},
				}
				_, err := announcementadmin.New(repo, nowFixed).Get(context.Background(), 1)
				assert.ErrorIs(t, err, tc.wantErr)
			})
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("告知の更新", func(t *testing.T) {
		dbErr := errors.New("db")
		cases := []struct {
			name    string
			repoErr error
			wantErr error
		}{
			{
				name:    "更新対象が存在するとき、エラーにならない",
				repoErr: nil,
				wantErr: nil,
			},
			{
				name:    "告知が見つからないとき、ErrNotFound になる",
				repoErr: port.ErrNotFound,
				wantErr: announcementadmin.ErrNotFound,
			},
			{
				name:    "想定外のエラーが起きたとき、そのエラーがそのまま返る",
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
	})
}

func TestDelete(t *testing.T) {
	t.Run("告知の削除", func(t *testing.T) {
		dbErr := errors.New("db")
		cases := []struct {
			name    string
			repoErr error
			wantErr error
		}{
			{
				name:    "削除対象が存在するとき、エラーにならない",
				repoErr: nil,
				wantErr: nil,
			},
			{
				name:    "告知が見つからないとき、ErrNotFound になる",
				repoErr: port.ErrNotFound,
				wantErr: announcementadmin.ErrNotFound,
			},
			{
				name:    "想定外のエラーが起きたとき、そのエラーがそのまま返る",
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
	})
}

func TestUpsertTranslation(t *testing.T) {
	t.Run("翻訳の登録・更新", func(t *testing.T) {
		validationCases := []struct {
			name    string
			lang    string
			title   string
			body    string
			wantErr error
		}{
			{
				name:    "対応外 lang のとき、ErrUnsupportedLang になる",
				lang:    "fr",
				title:   "T",
				body:    "B",
				wantErr: announcementadmin.ErrUnsupportedLang,
			},
			{
				name:    "title が空のとき、ErrInvalidField になる",
				lang:    domain.LangJa,
				title:   "",
				body:    "B",
				wantErr: announcementadmin.ErrInvalidField,
			},
			{
				name:    "body が空のとき、ErrInvalidField になる",
				lang:    domain.LangJa,
				title:   "T",
				body:    "",
				wantErr: announcementadmin.ErrInvalidField,
			},
			{
				name:    "title が上限超過 (201 文字) のとき、ErrInvalidField になる",
				lang:    domain.LangJa,
				title:   strings.Repeat("あ", 201),
				body:    "B",
				wantErr: announcementadmin.ErrInvalidField,
			},
		}

		for _, tc := range validationCases {
			t.Run(tc.name, func(t *testing.T) {
				err := announcementadmin.New(&port.MockAnnouncementRepo{}, nowFixed).UpsertTranslation(context.Background(), 1, tc.lang, tc.title, tc.body)

				assert.ErrorIs(t, err, tc.wantErr)
			})
		}

		t.Run("title が上限ちょうど (200 文字) のとき、登録できる", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				UpsertTranslationFn: func(_ context.Context, _ int64, _, _, _ string) error {
					return nil
				},
			}
			err := announcementadmin.New(repo, nowFixed).UpsertTranslation(context.Background(), 1, domain.LangJa, strings.Repeat("あ", 200), "B")

			require.NoError(t, err)
		})

		t.Run("告知が見つからないとき、ErrNotFound になる", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				UpsertTranslationFn: func(_ context.Context, _ int64, _ string, _ string, _ string) error {
					return port.ErrNotFound
				},
			}
			err := announcementadmin.New(repo, nowFixed).UpsertTranslation(context.Background(), 999, domain.LangJa, "T", "B")
			assert.ErrorIs(t, err, announcementadmin.ErrNotFound)
		})
	})
}
