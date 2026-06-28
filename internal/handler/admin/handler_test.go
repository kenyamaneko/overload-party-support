package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	announcementadmin "github.com/kenyamaneko/overload-party-support/internal/usecase/announcement_admin"
)

// newPostFormContext は urlencoded フォーム値を載せた gin.Context を組み立てる。
func newPostFormContext(form url.Values) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req
	return c
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

// TestCollectCreateTranslations はフォーム翻訳抽出の契約を固定する。
func TestCollectCreateTranslations(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		want    []domain.TranslationInput
		wantErr error
	}{
		{
			name: "en 両空は ja のみ",
			form: url.Values{"ja_title": {"題"}, "ja_body": {"本文"}},
			want: []domain.TranslationInput{
				{Lang: domain.LangJa, Title: "題", Body: "本文"},
			},
		},
		{
			name: "en 両在は ja + en",
			form: url.Values{"ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}, "en_body": {"B"}},
			want: []domain.TranslationInput{
				{Lang: domain.LangJa, Title: "題", Body: "本文"},
				{Lang: domain.LangEn, Title: "T", Body: "B"},
			},
		},
		{
			name:    "en title のみは ErrInvalidField",
			form:    url.Values{"ja_title": {"題"}, "ja_body": {"本文"}, "en_title": {"T"}},
			want:    nil,
			wantErr: announcementadmin.ErrInvalidField,
		},
		{
			name:    "en body のみは ErrInvalidField",
			form:    url.Values{"ja_title": {"題"}, "ja_body": {"本文"}, "en_body": {"B"}},
			want:    nil,
			wantErr: announcementadmin.ErrInvalidField,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectCreateTranslations(newPostFormContext(tc.form))
			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestParseAnnouncementID は announcementId パースの契約を固定する。
func TestParseAnnouncementID(t *testing.T) {
	cases := []struct {
		name       string
		param      string
		wantID     int64
		wantOK     bool
		wantStatus int
	}{
		{
			name:       "数値はパースして true",
			param:      "42",
			wantID:     42,
			wantOK:     true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "非数値は 404 で false",
			param:      "abc",
			wantID:     0,
			wantOK:     false,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "空は 404 で false",
			param:      "",
			wantID:     0,
			wantOK:     false,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "announcementId", Value: tc.param}}

			gotID, gotOK := parseAnnouncementID(c)

			assert.Equal(t, tc.wantID, gotID)
			assert.Equal(t, tc.wantOK, gotOK)
			assert.Equal(t, tc.wantStatus, w.Code)
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
