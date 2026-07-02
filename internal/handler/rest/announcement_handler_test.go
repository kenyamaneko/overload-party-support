package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

func newAnnouncementEngine(h *rest.AnnouncementHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/support/announcements", h.List)
	r.GET("/api/v1/support/announcements/:announcementId", h.GetDetail)
	return r
}

// 仕様 (data/openapi.yaml): GET /api/v1/support/announcements は lang 必須。
func TestAnnouncementList_LangValidation(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "lang=ja",
			query:      "?lang=ja",
			wantStatus: http.StatusOK,
		},
		{
			name:       "lang=en",
			query:      "?lang=en",
			wantStatus: http.StatusOK,
		},
		{
			name:       "lang 欠落",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "lang 対応外",
			query:      "?lang=fr",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]domain.AnnouncementSummary, error) {
					return []domain.AnnouncementSummary{}, nil
				},
			}
			h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/support/announcements"+tc.query, nil)
			w := httptest.NewRecorder()
			newAnnouncementEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

// 仕様: List レスポンスは 0 件でも nil でなく空配列を返す。
func TestAnnouncementList_EmptyArrayResponse(t *testing.T) {
	repo := &port.MockAnnouncementRepo{
		ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]domain.AnnouncementSummary, error) {
			return nil, nil
		},
	}
	h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/support/announcements?lang=ja", nil)
	w := httptest.NewRecorder()
	newAnnouncementEngine(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apisupport.AnnouncementListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Announcements)
	assert.Empty(t, resp.Announcements)
}

func TestAnnouncementList_ResponseFields(t *testing.T) {
	pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	repo := &port.MockAnnouncementRepo{
		ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]domain.AnnouncementSummary, error) {
			return []domain.AnnouncementSummary{
				{AnnouncementID: 1, Type: domain.TypeInfo, Title: "一件目", PublishedAt: pub},
				{AnnouncementID: 2, Type: domain.TypeMaintenance, Title: "二件目", PublishedAt: pub},
			}, nil
		},
	}
	h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/support/announcements?lang=ja", nil)
	w := httptest.NewRecorder()
	newAnnouncementEngine(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apisupport.AnnouncementListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	want := []apisupport.AnnouncementSummary{
		{AnnouncementID: 1, Type: apisupport.AnnouncementTypeInfo, Title: "一件目", PublishedAt: pub},
		{AnnouncementID: 2, Type: apisupport.AnnouncementTypeMaintenance, Title: "二件目", PublishedAt: pub},
	}
	assert.Equal(t, want, resp.Announcements)
}

func TestAnnouncementGetDetail_ResponseFields(t *testing.T) {
	pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		detail *domain.AnnouncementDetail
		want   apisupport.AnnouncementDetail
	}{
		{
			name:   "published_at ありは全フィールドを wire へ透過",
			detail: &domain.AnnouncementDetail{AnnouncementID: 7, Type: domain.TypeMaintenance, Title: "メンテ", Body: "本文", PublishedAt: &pub},
			want:   apisupport.AnnouncementDetail{AnnouncementID: 7, Type: apisupport.AnnouncementTypeMaintenance, Title: "メンテ", Body: "本文", PublishedAt: &pub},
		},
		{
			name:   "published_at なしは null で透過",
			detail: &domain.AnnouncementDetail{AnnouncementID: 8, Type: domain.TypeInfo, Title: "案内", Body: "本文", PublishedAt: nil},
			want:   apisupport.AnnouncementDetail{AnnouncementID: 8, Type: apisupport.AnnouncementTypeInfo, Title: "案内", Body: "本文", PublishedAt: nil},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(_ context.Context, _ int64, _ string) (*domain.AnnouncementDetail, error) {
					return tc.detail, nil
				},
			}
			h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/support/announcements/1?lang=ja", nil)
			w := httptest.NewRecorder()
			newAnnouncementEngine(h).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var resp apisupport.AnnouncementDetail
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, tc.want, resp)
		})
	}
}

// 仕様 (FEATURE_SPEC §10 / data/openapi.yaml): GetDetail のエラー分類を HTTP に変換する。
func TestAnnouncementGetDetail(t *testing.T) {
	dbErr := errors.New("db lost")
	okDetail := &domain.AnnouncementDetail{AnnouncementID: 1, Type: domain.TypeInfo}

	cases := []struct {
		name       string
		id         string
		query      string
		detail     *domain.AnnouncementDetail
		repoErr    error
		wantStatus int
	}{
		{
			name:       "正常",
			id:         "1",
			query:      "?lang=ja",
			detail:     okDetail,
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			id:         "1",
			query:      "?lang=ja",
			repoErr:    port.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "非数値 ID は 404",
			id:         "abc",
			query:      "?lang=ja",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "DB 障害は 500",
			id:         "1",
			query:      "?lang=ja",
			repoErr:    dbErr,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "lang 欠落は 400",
			id:         "1",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "lang 対応外は 400",
			id:         "1",
			query:      "?lang=fr",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(_ context.Context, _ int64, _ string) (*domain.AnnouncementDetail, error) {
					return tc.detail, tc.repoErr
				},
			}
			h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/support/announcements/"+tc.id+tc.query, nil)
			w := httptest.NewRecorder()
			newAnnouncementEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}
