package admin

import (
	"context"
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

// TestParseDatetimeLocal は datetime-local パースの契約を固定する。
func TestParseDatetimeLocal(t *testing.T) {
	valid := time.Date(2026, 4, 20, 10, 30, 0, 0, time.UTC)
	cases := []struct {
		name    string
		input   string
		want    *time.Time
		wantErr error
	}{
		{
			name:    "空文字は nil",
			input:   "",
			want:    nil,
			wantErr: nil,
		},
		{
			name:    "正常な datetime-local は UTC で解釈",
			input:   "2026-04-20T10:30",
			want:    &valid,
			wantErr: nil,
		},
		{
			name:    "区切り文字違いは ErrInvalidField",
			input:   "2026/04/20 10:30",
			want:    nil,
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name:    "時刻欠落は ErrInvalidField",
			input:   "2026-04-20",
			want:    nil,
			wantErr: announcementadmin.ErrInvalidField,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDatetimeLocal(tc.input)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.want, got)
		})
	}
}

// URL パス・モック値・期待値の間で一致が必要な announcement ID 群。片側だけの書き換えを防ぐため定数で束ねる。
const (
	createdAnnouncementID int64 = 10
	targetAnnouncementID  int64 = 7
	deletedAnnouncementID int64 = 42
)

func TestCreateRoute_CreatesTranslationsAndRedirects(t *testing.T) {
	cases := []struct {
		name             string
		form             url.Values
		wantTranslations []domain.TranslationInput
	}{
		{
			name: "en 両空は ja 翻訳のみで作成",
			form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}},
			wantTranslations: []domain.TranslationInput{
				{Lang: domain.LangJa, Title: "題", Body: "本文"},
			},
		},
		{
			name: "en 両在は ja + en 翻訳で作成",
			form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}, "en_body": {"B"}},
			wantTranslations: []domain.TranslationInput{
				{Lang: domain.LangJa, Title: "題", Body: "本文"},
				{Lang: domain.LangEn, Title: "T", Body: "B"},
			},
		},
	}

	for _, tc := range cases {
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
		})
	}
}

func TestCreateRoute_RejectsHalfEnTranslation(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
	}{
		{
			name: "en title のみは 400",
			form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}},
		},
		{
			name: "en body のみは 400",
			form: url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_body": {"B"}},
		},
	}

	for _, tc := range cases {
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
}

func TestDeleteRoute_DeletesAndRedirectsToList(t *testing.T) {
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
}

func TestDeleteRoute_ReturnsNotFoundForNonNumericID(t *testing.T) {
	// DeleteFn を未設定にすることで、呼ばれた瞬間に panic して「usecase に到達しない」ことを担保する (MockAnnouncementRepo 契約)
	repo := &port.MockAnnouncementRepo{}

	req := httptest.NewRequest(http.MethodPost, "/admin/announcements/abc/delete", nil)
	w := httptest.NewRecorder()
	newAdminEngine(t, repo).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListRoute_RendersSeededTitle(t *testing.T) {
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
}

func TestListRoute_RejectsUnknownStatusFilter(t *testing.T) {
	// ListFn を未設定にすることで、呼ばれた瞬間に panic して「repo に到達しない」ことを担保する (MockAnnouncementRepo 契約)
	repo := &port.MockAnnouncementRepo{}

	req := httptest.NewRequest(http.MethodGet, "/admin/announcements?status=bogus", nil)
	w := httptest.NewRecorder()
	newAdminEngine(t, repo).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), announcementadmin.ErrInvalidStatusFilter.Error())
}

func TestShowEditRoute_RendersJaTitle(t *testing.T) {
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
}

func TestShowEditRoute_ReturnsNotFoundForUnknownID(t *testing.T) {
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

func TestUpdateRoute_RedirectsToList(t *testing.T) {
	w := httptest.NewRecorder()
	newAdminEngine(t, newUpdateRepoReturning(nil)).ServeHTTP(w, newUpdateRequest(t))

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/announcements", w.Header().Get("Location"))
}

func TestUpdateRoute_HTMXRedirectsToList(t *testing.T) {
	req := newUpdateRequest(t)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	newAdminEngine(t, newUpdateRepoReturning(nil)).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/admin/announcements", w.Header().Get("HX-Redirect"))
}

func TestUpdateRoute_ReturnsNotFoundForUnknownID(t *testing.T) {
	w := httptest.NewRecorder()
	newAdminEngine(t, newUpdateRepoReturning(port.ErrNotFound)).ServeHTTP(w, newUpdateRequest(t))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), announcementadmin.ErrNotFound.Error())
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

func TestUpsertTranslationRoute_RedirectsToEdit(t *testing.T) {
	w := httptest.NewRecorder()
	newAdminEngine(t, newUpsertRepoReturning(nil)).ServeHTTP(w, newUpsertTranslationRequest(t))

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), w.Header().Get("Location"))
}

func TestUpsertTranslationRoute_HTMXRedirectsToEdit(t *testing.T) {
	req := newUpsertTranslationRequest(t)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	newAdminEngine(t, newUpsertRepoReturning(nil)).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, fmt.Sprintf("/admin/announcements/%d", targetAnnouncementID), w.Header().Get("HX-Redirect"))
}

func TestUpsertTranslationRoute_ReturnsNotFoundForUnknownID(t *testing.T) {
	w := httptest.NewRecorder()
	newAdminEngine(t, newUpsertRepoReturning(port.ErrNotFound)).ServeHTTP(w, newUpsertTranslationRequest(t))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), announcementadmin.ErrNotFound.Error())
}

func TestDeleteRoute_HTMXRemovesRow(t *testing.T) {
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
}

// TestToAdminListItem は一覧ビューモデル変換の契約を固定する。
func TestToAdminListItem(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	cases := []struct {
		name           string
		translations   []domain.Translation
		publishedAt    *time.Time
		wantJaTitle    string
		wantLangsLabel string
		wantState      string
		wantErr        bool
	}{
		{
			name: "ja + en は ja タイトルと両言語ラベル",
			translations: []domain.Translation{
				{Lang: domain.LangJa, Title: "日本語題"},
				{Lang: domain.LangEn, Title: "English"},
			},
			publishedAt:    &past,
			wantJaTitle:    "日本語題",
			wantLangsLabel: "ja, en",
			wantState:      domain.StatePublished,
		},
		{
			name:           "ja のみは単一言語ラベル",
			translations:   []domain.Translation{{Lang: domain.LangJa, Title: "日本語題"}},
			publishedAt:    nil,
			wantJaTitle:    "日本語題",
			wantLangsLabel: "ja",
			wantState:      domain.StateDraft,
		},
		{
			name:         "ja 翻訳欠落はエラー",
			translations: []domain.Translation{{Lang: domain.LangEn, Title: "English"}},
			publishedAt:  &past,
			wantErr:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toAdminListItem(domain.AnnouncementWithTranslations{
				Announcement: domain.Announcement{PublishedAt: tc.publishedAt},
				Translations: tc.translations,
			}, now)

			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.wantJaTitle, got.JaTitle)
			assert.Equal(t, tc.wantLangsLabel, got.LangsLabel)
			assert.Equal(t, tc.wantState, got.State)
		})
	}
}
