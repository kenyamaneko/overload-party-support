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

	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/service/announcement"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

func newAnnouncementEngine(h *rest.AnnouncementHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/internal/v1/announcements", h.List)
	r.GET("/internal/v1/announcements/:announcementId", h.GetDetail)
	return r
}

// 仕様 (API_REFERENCE): GET /internal/v1/announcements は lang 必須。欠落・対応外は 400。
func TestAnnouncementList_仕様_langバリデーション(t *testing.T) {
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
				ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]apisupport.AnnouncementSummary, error) {
					return []apisupport.AnnouncementSummary{}, nil
				},
			}
			h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

			req := httptest.NewRequest(http.MethodGet, "/internal/v1/announcements"+tc.query, nil)
			w := httptest.NewRecorder()
			newAnnouncementEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

// 仕様: List レスポンスは {"announcements": [...]} 形式、0 件でも nil でなく空配列。
func TestAnnouncementList_仕様_空配列レスポンス(t *testing.T) {
	repo := &port.MockAnnouncementRepo{
		ListPublishedFn: func(_ context.Context, _ string, _ time.Time) ([]apisupport.AnnouncementSummary, error) {
			return nil, nil
		},
	}
	h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/announcements?lang=ja", nil)
	w := httptest.NewRecorder()
	newAnnouncementEngine(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apisupport.AnnouncementListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Announcements)
	assert.Empty(t, resp.Announcements)
}

// 仕様 (FEATURE_SPEC §10 / API_REFERENCE): GetDetail のエラー分類を HTTP に変換する。
func TestAnnouncementGetDetail_仕様_HTTPマッピング(t *testing.T) {
	dbErr := errors.New("db lost")

	cases := []struct {
		name       string
		id         string
		lang       string
		repoErr    error
		wantStatus int
	}{
		{
			name:       "正常",
			id:         "1",
			lang:       "ja",
			repoErr:    nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			id:         "1",
			lang:       "ja",
			repoErr:    port.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "非数値 ID は 404",
			id:         "abc",
			lang:       "ja",
			repoErr:    nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "DB 障害は 500",
			id:         "1",
			lang:       "ja",
			repoErr:    dbErr,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "lang 欠落は 400",
			id:         "1",
			lang:       "",
			repoErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "lang 対応外は 400",
			id:         "1",
			lang:       "fr",
			repoErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(_ context.Context, _ int64, _ string) (*apisupport.AnnouncementDetail, error) {
					if tc.repoErr != nil {
						return nil, tc.repoErr
					}
					return &apisupport.AnnouncementDetail{AnnouncementID: 1, Type: apisupport.TypeInfo}, nil
				},
			}
			h := rest.NewAnnouncementHandler(announcement.New(repo, time.Now))

			url := "/internal/v1/announcements/" + tc.id
			if tc.lang != "" {
				url += "?lang=" + tc.lang
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			newAnnouncementEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}
