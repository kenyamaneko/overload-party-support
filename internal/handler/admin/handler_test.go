package admin

import (
	"context"
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
	g.POST("/announcements", h.Create)
	g.POST("/announcements/:announcementId/delete", h.Delete)
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

func TestCreateRoute(t *testing.T) {
	cases := []struct {
		name             string
		form             url.Values
		createID         int64 // usecase.Create が採番する想定 ID。成功ケースのリダイレクト先に現れる
		wantStatus       int
		wantLocation     string
		wantTranslations []domain.TranslationInput
	}{
		{
			name:         "en 両空は ja 翻訳のみで作成しリダイレクト",
			form:         url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}},
			createID:     10,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/admin/announcements/10",
			wantTranslations: []domain.TranslationInput{
				{Lang: domain.LangJa, Title: "題", Body: "本文"},
			},
		},
		{
			name:         "en 両在は ja + en 翻訳で作成しリダイレクト",
			form:         url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}, "en_body": {"B"}},
			createID:     10,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/admin/announcements/10",
			wantTranslations: []domain.TranslationInput{
				{Lang: domain.LangJa, Title: "題", Body: "本文"},
				{Lang: domain.LangEn, Title: "T", Body: "B"},
			},
		},
		{
			name:       "en title のみは 400 で usecase に到達しない",
			form:       url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "en body のみは 400 で usecase に到達しない",
			form:       url.Values{"type": {domain.TypeInfo}, "ja_title": {"題"}, "ja_body": {"本文"}, "en_body": {"B"}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotParams domain.CreateAnnouncementParams
			repo := &port.MockAnnouncementRepo{
				CreateFn: func(_ context.Context, params domain.CreateAnnouncementParams) (int64, error) {
					gotParams = params
					return tc.createID, nil
				},
			}

			req := httptest.NewRequest(http.MethodPost, "/admin/announcements", strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantLocation, w.Header().Get("Location"))
			assert.Equal(t, tc.wantTranslations, gotParams.Translations)
		})
	}
}

func TestDeleteRoute(t *testing.T) {
	cases := []struct {
		name          string
		idParam       string
		wantStatus    int
		wantLocation  string
		wantDeletedID int64
	}{
		{
			name:          "数値 ID は usecase に渡され一覧へリダイレクト",
			idParam:       "42",
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/admin/announcements",
			wantDeletedID: 42,
		},
		{
			name:          "非数値 ID は 404 で usecase に到達しない",
			idParam:       "abc",
			wantStatus:    http.StatusNotFound,
			wantLocation:  "",
			wantDeletedID: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotDeletedID int64
			repo := &port.MockAnnouncementRepo{
				DeleteFn: func(_ context.Context, announcementID int64) error {
					gotDeletedID = announcementID
					return nil
				},
			}

			req := httptest.NewRequest(http.MethodPost, "/admin/announcements/"+tc.idParam+"/delete", nil)
			w := httptest.NewRecorder()
			newAdminEngine(t, repo).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantLocation, w.Header().Get("Location"))
			assert.Equal(t, tc.wantDeletedID, gotDeletedID)
		})
	}
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
