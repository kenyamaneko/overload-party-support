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

func TestAnnouncementList(t *testing.T) {
	t.Run("公開告知一覧 API", func(t *testing.T) {
		cases := []struct {
			name       string
			query      string
			wantStatus int
		}{
			{
				name:       "lang=ja のとき、200 になる",
				query:      "?lang=ja",
				wantStatus: http.StatusOK,
			},
			{
				name:       "lang=en のとき、200 になる",
				query:      "?lang=en",
				wantStatus: http.StatusOK,
			},
			{
				name:       "lang が欠落するとき、400 になる",
				query:      "",
				wantStatus: http.StatusBadRequest,
			},
			{
				name:       "lang が対応外 (fr) のとき、400 になる",
				query:      "?lang=fr",
				wantStatus: http.StatusBadRequest,
			},
		}

		for _, tc := range cases {
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

		t.Run("0 件でも null でなく空配列を返す", func(t *testing.T) {
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
		})

		t.Run("各 summary フィールドが wire に反映される", func(t *testing.T) {
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
		})
	})
}

func TestAnnouncementGetDetail(t *testing.T) {
	t.Run("公開告知詳細 API", func(t *testing.T) {
		pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

		fieldCases := []struct {
			name   string
			detail *domain.AnnouncementDetail
			want   apisupport.AnnouncementDetail
		}{
			{
				name:   "published_at ありのとき、全フィールドが wire へ透過される",
				detail: &domain.AnnouncementDetail{AnnouncementID: 7, Type: domain.TypeMaintenance, Title: "メンテ", Body: "本文", PublishedAt: &pub},
				want:   apisupport.AnnouncementDetail{AnnouncementID: 7, Type: apisupport.AnnouncementTypeMaintenance, Title: "メンテ", Body: "本文", PublishedAt: &pub},
			},
			{
				name:   "published_at なしのとき、published_at は null で透過される",
				detail: &domain.AnnouncementDetail{AnnouncementID: 8, Type: domain.TypeInfo, Title: "案内", Body: "本文", PublishedAt: nil},
				want:   apisupport.AnnouncementDetail{AnnouncementID: 8, Type: apisupport.AnnouncementTypeInfo, Title: "案内", Body: "本文", PublishedAt: nil},
			},
		}

		for _, tc := range fieldCases {
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

		dbErr := errors.New("db lost")
		okDetail := &domain.AnnouncementDetail{AnnouncementID: 1, Type: domain.TypeInfo}

		statusCases := []struct {
			name       string
			id         string
			query      string
			detail     *domain.AnnouncementDetail
			repoErr    error
			wantStatus int
		}{
			{
				name:       "正常のとき、200 になる",
				id:         "1",
				query:      "?lang=ja",
				detail:     okDetail,
				wantStatus: http.StatusOK,
			},
			{
				name:       "not found のとき、404 になる",
				id:         "1",
				query:      "?lang=ja",
				repoErr:    port.ErrNotFound,
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "非数値 ID のとき、404 になる",
				id:         "abc",
				query:      "?lang=ja",
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "DB 障害のとき、500 になる",
				id:         "1",
				query:      "?lang=ja",
				repoErr:    dbErr,
				wantStatus: http.StatusInternalServerError,
			},
			{
				name:       "lang が欠落するとき、400 になる",
				id:         "1",
				query:      "",
				wantStatus: http.StatusBadRequest,
			},
			{
				name:       "lang が対応外のとき、400 になる",
				id:         "1",
				query:      "?lang=fr",
				wantStatus: http.StatusBadRequest,
			},
		}

		for _, tc := range statusCases {
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
	})
}
