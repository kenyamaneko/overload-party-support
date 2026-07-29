package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/config"
	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/usecase/announcement_admin"
)

// fixedAdminNow は admin route テストの時刻依存を排除するための固定時刻。
var fixedAdminNow = time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

// newAdminEngine は repo モックを差し込んだ admin ルータを組み立てる。
func newAdminEngine(t *testing.T, repo *port.MockAnnouncementRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	now := func() time.Time { return fixedAdminNow }
	h, err := NewHandler(announcementadmin.New(repo, now), now)
	require.NoError(t, err)

	r := gin.New()
	g := r.Group("/admin", AuthMiddleware(config.EnvLocal))
	g.GET("/announcements", h.List)
	g.POST("/announcements", h.Create)
	g.GET("/announcements/:announcementId", h.ShowEdit)
	g.POST("/announcements/:announcementId", h.Update)
	g.POST("/announcements/:announcementId/delete", h.Delete)
	g.POST("/announcements/:announcementId/translations/:lang", h.UpsertTranslation)
	return r
}

func TestParseDatetimeLocal(t *testing.T) {
	t.Run("datetime-local のパース", func(t *testing.T) {
		valid := time.Date(2026, 4, 20, 10, 30, 0, 0, time.UTC)

		validCases := []struct {
			name  string
			input string
			want  *time.Time
		}{
			{
				name:  "空文字のとき、nil を返す",
				input: "",
				want:  nil,
			},
			{
				name:  "datetime-local 形式のとき、UTC の時刻を返す",
				input: "2026-04-20T10:30",
				want:  &valid,
			},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := parseDatetimeLocal(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}

		invalidCases := []struct {
			name  string
			input string
		}{
			{
				name:  "区切り文字が違うとき、ErrInvalidField になる",
				input: "2026/04/20 10:30",
			},
			{
				name:  "時刻が欠落するとき、ErrInvalidField になる",
				input: "2026-04-20",
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := parseDatetimeLocal(tc.input)
				assert.ErrorIs(t, err, announcementadmin.ErrInvalidField)
				assert.Nil(t, got)
			})
		}
	})
}

// URL パス・モック値・期待値の間で一致が必要な announcement ID 群。片側だけの書き換えを防ぐため定数で束ねる。
const (
	createdAnnouncementID int64 = 10
	targetAnnouncementID  int64 = 7
	deletedAnnouncementID int64 = 42
)

func TestCreateRoute(t *testing.T) {
	t.Run("告知の作成", func(t *testing.T) {
		wantPublished := time.Date(2026, 4, 20, 9, 30, 0, 0, time.UTC)
		wantExpires := time.Date(2026, 5, 20, 9, 30, 0, 0, time.UTC)

		successCases := []struct {
			name             string
			form             url.Values
			wantTranslations []domain.TranslationInput
			wantPublishedAt  *time.Time
			wantExpiresAt    *time.Time
		}{
			{
				name: "en が両方空のとき、ja 翻訳のみで作成され 303 でリダイレクトする",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}},
				wantTranslations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "題", Body: "本文"},
				},
			},
			{
				name: "en が両方あるとき、ja + en 翻訳で作成され 303 でリダイレクトする",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}, "en_body": {"B"}},
				wantTranslations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "題", Body: "本文"},
					{Lang: domain.LangEn, Title: "T", Body: "B"},
				},
			},
			{
				name: "公開開始を空で作成すると、公開開始なし (下書き) として保存される",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "published_at": {""}},
				wantTranslations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "題", Body: "本文"},
				},
			},
			{
				name: "公開開始と公開終了を指定して作成すると、その UTC 時刻で保存される",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "published_at": {"2026-04-20T09:30"}, "expires_at": {"2026-05-20T09:30"}},
				wantTranslations: []domain.TranslationInput{
					{Lang: domain.LangJa, Title: "題", Body: "本文"},
				},
				wantPublishedAt: &wantPublished,
				wantExpiresAt:   &wantExpires,
			},
		}

		for _, tc := range successCases {
			t.Run(tc.name, func(t *testing.T) {
				var gotParams domain.CreateAnnouncementParams
				repo := &port.MockAnnouncementRepo{
					CreateFn: func(_ context.Context, params domain.CreateAnnouncementParams) (int64, error) {
						gotParams = params
						return createdAnnouncementID, nil
					},
				}

				req := httptest.NewRequest(http.MethodPost, "/admin/announcements", strings.NewReader(tc.form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				w := httptest.NewRecorder()
				newAdminEngine(t, repo).ServeHTTP(w, req)

				assert.Equal(t, http.StatusSeeOther, w.Code)
				assert.Equal(t, fmt.Sprintf("/admin/announcements/%d", createdAnnouncementID), w.Header().Get("Location"))
				assert.Equal(t, tc.wantTranslations, gotParams.Translations)
				assert.Equal(t, tc.wantPublishedAt, gotParams.PublishedAt)
				assert.Equal(t, tc.wantExpiresAt, gotParams.ExpiresAt)
			})
		}

		rejectCases := []struct {
			name string
			form url.Values
		}{
			{
				name: "en title のみのとき、400 になる",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}},
			},
			{
				name: "en body のみのとき、400 になる",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_body": {"B"}},
			},
			{
				name: "公開開始の形式が不正 (2026/04/20 09:30) のとき、400 になり保存されない",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "published_at": {"2026/04/20 09:30"}},
			},
			{
				name: "公開終了の形式が不正 (2026-04-20) のとき、400 になり保存されない",
				form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "expires_at": {"2026-04-20"}},
			},
		}

		for _, tc := range rejectCases {
			t.Run(tc.name, func(t *testing.T) {
				// CreateFn を未設定にすることで、呼ばれた瞬間に panic して「usecase に到達しない」ことを担保する (MockAnnouncementRepo 契約)
				repo := &port.MockAnnouncementRepo{}

				req := httptest.NewRequest(http.MethodPost, "/admin/announcements", strings.NewReader(tc.form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				w := httptest.NewRecorder()
				newAdminEngine(t, repo).ServeHTTP(w, req)

				assert.Equal(t, http.StatusBadRequest, w.Code)
			})
		}
	})
}

func TestListRoute(t *testing.T) {
	t.Run("告知一覧の表示", func(t *testing.T) {
		t.Run("seed した告知の ja タイトルが一覧に表示される", func(t *testing.T) {
			published := fixedAdminNow.Add(-time.Hour)
			seeded := domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{AnnouncementID: 5, Type: domain.TypeInfo, PublishedAt: &published},
				Translations: []domain.Translation{
					{Lang: domain.LangJa, Title: "一覧に出る題", Body: "本文"},
					{Lang: domain.LangEn, Title: "listed", Body: "b"},
				},
			}
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return []domain.AnnouncementWithTranslations{seeded}, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "一覧に出る題")
		})

		t.Run("未知の status フィルタのとき、400 になる", func(t *testing.T) {
			// ListFn を未設定にすることで、呼ばれた瞬間に panic して「repo に到達しない」ことを担保する (MockAnnouncementRepo 契約)
			repo := &port.MockAnnouncementRepo{}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements?status=bogus", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), announcementadmin.ErrInvalidStatusFilter.Error())
		})

		t.Run("一覧の取得で想定外のエラーが起きたとき、500 になる", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return nil, errors.New("db lost")
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("ja 翻訳の無い告知が一覧に含まれるとき、500 になる", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return []domain.AnnouncementWithTranslations{
						{
							Announcement: domain.Announcement{AnnouncementID: 9, Type: domain.TypeInfo},
							Translations: []domain.Translation{{Lang: domain.LangEn, Title: "en only", Body: "b"}},
						},
					}, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("公開中と下書きの告知があるとき、state 列に published と draft が表示される", func(t *testing.T) {
			published := fixedAdminNow.Add(-time.Hour)
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return []domain.AnnouncementWithTranslations{
						{
							Announcement: domain.Announcement{AnnouncementID: 1, Type: domain.TypeInfo, PublishedAt: &published},
							Translations: []domain.Translation{{Lang: domain.LangJa, Title: "公開中の題", Body: "b"}},
						},
						{
							Announcement: domain.Announcement{AnnouncementID: 2, Type: domain.TypeInfo},
							Translations: []domain.Translation{{Lang: domain.LangJa, Title: "下書きの題", Body: "b"}},
						},
					}, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "<code>published</code>")
			assert.Contains(t, w.Body.String(), "<code>draft</code>")
		})

		t.Run("ja と en の翻訳がある告知は、翻訳列に ja, en と表示される", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return []domain.AnnouncementWithTranslations{
						{
							Announcement: domain.Announcement{AnnouncementID: 3, Type: domain.TypeInfo},
							Translations: []domain.Translation{
								{Lang: domain.LangJa, Title: "題", Body: "b"},
								{Lang: domain.LangEn, Title: "title", Body: "b"},
							},
						},
					}, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "ja, en")
		})

		t.Run("ja のみ翻訳がある告知は、翻訳列に ja とだけ表示される", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return []domain.AnnouncementWithTranslations{
						{
							Announcement: domain.Announcement{AnnouncementID: 4, Type: domain.TypeInfo},
							Translations: []domain.Translation{{Lang: domain.LangJa, Title: "題", Body: "b"}},
						},
					}, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "<small>ja</small>")
		})

		t.Run("告知が 0 件のとき、「該当するお知らせがありません。」と表示される", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				ListFn: func(_ context.Context, _ *string, _ time.Time) ([]domain.AnnouncementWithTranslations, error) {
					return []domain.AnnouncementWithTranslations{}, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "該当するお知らせがありません。")
		})
	})
}

func TestShowEditRoute(t *testing.T) {
	t.Run("告知編集画面の表示", func(t *testing.T) {
		t.Run("存在する告知のとき、ja タイトルが表示される", func(t *testing.T) {
			existing := &domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{AnnouncementID: targetAnnouncementID, Type: domain.TypeInfo},
				Translations: []domain.Translation{{Lang: domain.LangJa, Title: "編集対象の題", Body: "本文"}},
			}
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return existing, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "編集対象の題")
		})

		t.Run("存在しない ID のとき、404 になる", func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return nil, port.ErrNotFound
				},
			}

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), announcementadmin.ErrNotFound.Error())
		})

		t.Run("公開開始が設定済みで公開終了が未設定のとき、公開開始欄にその日時・公開終了欄は空で表示される", func(t *testing.T) {
			published := time.Date(2026, 4, 20, 9, 30, 0, 0, time.UTC)
			existing := &domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{AnnouncementID: targetAnnouncementID, Type: domain.TypeInfo, PublishedAt: &published},
				Translations: []domain.Translation{{Lang: domain.LangJa, Title: "題", Body: "本文"}},
			}
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return existing, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), `name="published_at" value="2026-04-20T09:30"`)
			assert.Contains(t, w.Body.String(), `name="expires_at" value=""`)
		})

		t.Run("en 翻訳が未作成のとき、en の欄に 未作成 と表示される", func(t *testing.T) {
			existing := &domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{AnnouncementID: targetAnnouncementID, Type: domain.TypeInfo},
				Translations: []domain.Translation{{Lang: domain.LangJa, Title: "題", Body: "本文"}},
			}
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return existing, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "未作成")
		})

		t.Run("en 翻訳があるとき、そのタイトルが入力欄に表示される", func(t *testing.T) {
			existing := &domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{AnnouncementID: targetAnnouncementID, Type: domain.TypeInfo},
				Translations: []domain.Translation{
					{Lang: domain.LangJa, Title: "題", Body: "本文"},
					{Lang: domain.LangEn, Title: "en 翻訳表示確認用タイトル", Body: "body"},
				},
			}
			repo := &port.MockAnnouncementRepo{
				GetWithTranslationsFn: func(_ context.Context, _ int64) (*domain.AnnouncementWithTranslations, error) {
					return existing, nil
				},
			}

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "en 翻訳表示確認用タイトル")
		})
	})
}

// newUpdateRequest は type のみ指定した更新 form の POST リクエストを組み立てる。
func newUpdateRequest(t *testing.T) *http.Request {
	t.Helper()
	form := url.Values{"type": {domain.TypeInfo}}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// newUpdateRepoReturning は Update が指定エラーを返す repo モックを組み立てる。
func newUpdateRepoReturning(repoErr error) *port.MockAnnouncementRepo {
	return &port.MockAnnouncementRepo{
		UpdateFn: func(_ context.Context, _ int64, _ domain.UpdateAnnouncementParams) error {
			return repoErr
		},
	}
}

func TestUpdateRoute(t *testing.T) {
	t.Run("告知の更新", func(t *testing.T) {
		t.Run("更新すると 303 で一覧へリダイレクトする", func(t *testing.T) {
			w := httptest.NewRecorder()
			newAdminEngine(t, newUpdateRepoReturning(nil)).ServeHTTP(w, newUpdateRequest(t))

			assert.Equal(t, http.StatusSeeOther, w.Code)
			assert.Equal(t, "/admin/announcements", w.Header().Get("Location"))
		})

		t.Run("HTMX リクエストのとき、200 + HX-Redirect で一覧へ誘導する", func(t *testing.T) {
			req := newUpdateRequest(t)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			newAdminEngine(t, newUpdateRepoReturning(nil)).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "/admin/announcements", w.Header().Get("HX-Redirect"))
		})

		t.Run("存在しない ID のとき、404 になる", func(t *testing.T) {
			w := httptest.NewRecorder()
			newAdminEngine(t, newUpdateRepoReturning(port.ErrNotFound)).ServeHTTP(w, newUpdateRequest(t))

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), announcementadmin.ErrNotFound.Error())
		})

		t.Run("公開開始を指定して更新すると、その UTC 時刻で保存される", func(t *testing.T) {
			var gotParams domain.UpdateAnnouncementParams
			repo := &port.MockAnnouncementRepo{
				UpdateFn: func(_ context.Context, _ int64, params domain.UpdateAnnouncementParams) error {
					gotParams = params
					return nil
				},
			}
			form := url.Values{"type": {domain.TypeInfo}, "published_at": {"2026-04-21T00:00"}}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			want := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
			assert.Equal(t, &want, gotParams.PublishedAt)
		})

		t.Run("種別に未知の値 (nope) を指定して更新すると、400 になる", func(t *testing.T) {
			form := url.Values{"type": {"nope"}}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			newAdminEngine(t, &port.MockAnnouncementRepo{}).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})
}

// newUpsertTranslationRequest は ja 翻訳の title / body を指定した upsert form の POST リクエストを組み立てる。
func newUpsertTranslationRequest(t *testing.T) *http.Request {
	t.Helper()
	form := url.Values{"title": {"題"}, "body": {"本文"}}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d/translations/ja", targetAnnouncementID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// newUpsertRepoReturning は UpsertTranslation が指定エラーを返す repo モックを組み立てる。
func newUpsertRepoReturning(repoErr error) *port.MockAnnouncementRepo {
	return &port.MockAnnouncementRepo{
		UpsertTranslationFn: func(_ context.Context, _ int64, _, _, _ string) error {
			return repoErr
		},
	}
}

func TestUpsertTranslationRoute(t *testing.T) {
	t.Run("翻訳の登録・更新", func(t *testing.T) {
		t.Run("登録すると 303 で編集画面へリダイレクトする", func(t *testing.T) {
			w := httptest.NewRecorder()
			newAdminEngine(t, newUpsertRepoReturning(nil)).ServeHTTP(w, newUpsertTranslationRequest(t))

			assert.Equal(t, http.StatusSeeOther, w.Code)
			assert.Equal(t, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), w.Header().Get("Location"))
		})

		t.Run("HTMX リクエストのとき、200 + HX-Redirect で編集画面へ誘導する", func(t *testing.T) {
			req := newUpsertTranslationRequest(t)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			newAdminEngine(t, newUpsertRepoReturning(nil)).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), w.Header().Get("HX-Redirect"))
		})

		t.Run("存在しない ID のとき、404 になる", func(t *testing.T) {
			w := httptest.NewRecorder()
			newAdminEngine(t, newUpsertRepoReturning(port.ErrNotFound)).ServeHTTP(w, newUpsertTranslationRequest(t))

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), announcementadmin.ErrNotFound.Error())
		})

		t.Run("対応外の言語 (fr) へ翻訳を登録すると、400 になる", func(t *testing.T) {
			form := url.Values{"title": {"題"}, "body": {"本文"}}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d/translations/fr", targetAnnouncementID), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			newAdminEngine(t, &port.MockAnnouncementRepo{}).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})
}

func TestDeleteRoute(t *testing.T) {
	t.Run("告知の削除", func(t *testing.T) {
		t.Run("削除すると 303 で一覧へリダイレクトする", func(t *testing.T) {
			var gotDeletedID int64
			repo := &port.MockAnnouncementRepo{
				DeleteFn: func(_ context.Context, announcementID int64) error {
					gotDeletedID = announcementID
					return nil
				},
			}

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d/delete", deletedAnnouncementID), nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			assert.Equal(t, "/admin/announcements", w.Header().Get("Location"))
			assert.Equal(t, deletedAnnouncementID, gotDeletedID)
		})

		t.Run("非数値 ID のとき、404 になる", func(t *testing.T) {
			// DeleteFn を未設定にすることで、呼ばれた瞬間に panic して「usecase に到達しない」ことを担保する (MockAnnouncementRepo 契約)
			repo := &port.MockAnnouncementRepo{}

			req := httptest.NewRequest(http.MethodPost, "/admin/announcements/abc/delete", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("HX-Target が row-42 のとき、200 で空ボディを返し行を消す", func(t *testing.T) {
			var gotDeletedID int64
			repo := &port.MockAnnouncementRepo{
				DeleteFn: func(_ context.Context, announcementID int64) error {
					gotDeletedID = announcementID
					return nil
				},
			}

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/announcements/%d/delete", deletedAnnouncementID), nil)
			req.Header.Set("HX-Target", fmt.Sprintf("row-%d", deletedAnnouncementID))
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Empty(t, w.Body.String())
			assert.Equal(t, deletedAnnouncementID, gotDeletedID)
		})
	})
}
