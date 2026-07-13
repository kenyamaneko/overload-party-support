package apisupportclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportclient"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 各 TestClient_<Endpoint> は、fake サーバ (httptest) を経由した実際の通信を介して
// (1) 成功 status のとき fake が返した body が typed 戻り値へ正しく decode されること、
// (2) error status のとき対応する sentinel が errors.Is で判別できることを検証する。

func TestClient_ListAnnouncements(t *testing.T) {
	t.Run("ListAnnouncements", func(t *testing.T) {
		t.Run("200 を受けたとき、fake が返した body が AnnouncementListResponse へ復元される", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
			srv.ListAnnouncementsFn = func(_ string) (int, any) {
				return http.StatusOK, apisupport.AnnouncementListResponse{
					Announcements: []apisupport.AnnouncementSummary{
						{AnnouncementID: 1, Type: apisupport.AnnouncementTypeInfo, Title: "T", PublishedAt: pub},
					},
				}
			}

			c := newTestClient(t, srv.URL())
			got, err := c.ListAnnouncements(context.Background(), "ja")

			require.NoError(t, err)
			require.Len(t, got.Announcements, 1)
			assert.Equal(t, int64(1), got.Announcements[0].AnnouncementID)
			assert.Equal(t, apisupport.AnnouncementTypeInfo, got.Announcements[0].Type)
			assert.Equal(t, "T", got.Announcements[0].Title)
			assert.True(t, pub.Equal(got.Announcements[0].PublishedAt))
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(_ string) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListAnnouncements(context.Background(), "ja")
			assertSentinel(t, err, apisupportclient.ErrBadRequest)
		})

		t.Run("401 を受けたとき、ErrUnauthorized になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(_ string) (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListAnnouncements(context.Background(), "ja")
			assertSentinel(t, err, apisupportclient.ErrUnauthorized)
		})
	})
}

func TestClient_GetAnnouncement(t *testing.T) {
	t.Run("GetAnnouncement", func(t *testing.T) {
		t.Run("200 を受けたとき、fake が返した body が AnnouncementDetail へ復元される", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.GetAnnouncementFn = func(announcementID int64, _ string) (int, any) {
				return http.StatusOK, apisupport.AnnouncementDetail{
					AnnouncementID: announcementID,
					Type:           apisupport.AnnouncementTypeMaintenance,
					Title:          "T",
					Body:           "B",
					PublishedAt:    nil,
				}
			}

			c := newTestClient(t, srv.URL())
			got, err := c.GetAnnouncement(context.Background(), 42, "ja")

			require.NoError(t, err)
			assert.Equal(t, int64(42), got.AnnouncementID)
			assert.Equal(t, apisupport.AnnouncementTypeMaintenance, got.Type)
			assert.Equal(t, "T", got.Title)
			assert.Equal(t, "B", got.Body)
			assert.Nil(t, got.PublishedAt)
		})

		cases := []struct {
			name       string
			status     int
			wantTarget error
		}{
			{
				name:       "400 を受けたとき、ErrBadRequest になる",
				status:     http.StatusBadRequest,
				wantTarget: apisupportclient.ErrBadRequest,
			},
			{
				name:       "404 を受けたとき、ErrNotFound になる",
				status:     http.StatusNotFound,
				wantTarget: apisupportclient.ErrNotFound,
			},
			{
				name:       "500 を受けたとき、ErrInternalServer になる",
				status:     http.StatusInternalServerError,
				wantTarget: apisupportclient.ErrInternalServer,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				srv := apisupportserverfake.NewServer()
				defer srv.Close()
				srv.GetAnnouncementFn = func(_ int64, _ string) (int, any) { return tc.status, nil }

				c := newTestClient(t, srv.URL())
				_, err := c.GetAnnouncement(context.Background(), 1, "ja")
				assertSentinel(t, err, tc.wantTarget)
			})
		}
	})
}

func TestClient_SubmitInquiry(t *testing.T) {
	t.Run("SubmitInquiry", func(t *testing.T) {
		t.Run("200 を受けたとき、fake が返した body が SubmitInquiryResult へ復元される", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.SubmitInquiryFn = func(_ apisupport.SubmitInquiryRequest) (int, any) {
				return http.StatusOK, apisupport.SubmitInquiryResult{InquiryID: 999}
			}

			c := newTestClient(t, srv.URL())
			got, err := c.SubmitInquiry(context.Background(), apisupport.SubmitInquiryRequest{})

			require.NoError(t, err)
			assert.Equal(t, int64(999), got.InquiryID)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.SubmitInquiryFn = func(_ apisupport.SubmitInquiryRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			// status mapping 検証のため request body の内容は無関係 (server fake は body を見ず status を返す)。
			_, err := c.SubmitInquiry(context.Background(), apisupport.SubmitInquiryRequest{})
			assertSentinel(t, err, apisupportclient.ErrBadRequest)
		})

		t.Run("写像外の status (418) を受けたとき、いずれの sentinel にもならないエラーになる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.SubmitInquiryFn = func(_ apisupport.SubmitInquiryRequest) (int, any) { return http.StatusTeapot, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.SubmitInquiry(context.Background(), apisupport.SubmitInquiryRequest{})

			require.Error(t, err)
			assert.NotErrorIs(t, err, apisupportclient.ErrBadRequest)
			assert.NotErrorIs(t, err, apisupportclient.ErrUnauthorized)
			assert.NotErrorIs(t, err, apisupportclient.ErrNotFound)
			assert.NotErrorIs(t, err, apisupportclient.ErrInternalServer)
		})
	})
}

func TestClient_RequestEditor(t *testing.T) {
	t.Run("リクエストエディタの適用", func(t *testing.T) {
		t.Run("WithRequestEditorFn で渡した editor が全リクエストに適用される", func(t *testing.T) {
			// header 注入の接続点として SDK が機能することを担保する。
			var gotHeader string
			spy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Get("X-Custom-Header")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"announcements":[]}`))
			}))
			defer spy.Close()

			c, err := apisupportclient.New(spy.URL,
				apisupportclient.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
					req.Header.Set("X-Custom-Header", "test-token")
					return nil
				}),
			)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")
			require.NoError(t, err)
			assert.Equal(t, "test-token", gotHeader)
		})
	})
}

func newTestClient(t *testing.T, baseURL string) *apisupportclient.Client {
	t.Helper()
	c, err := apisupportclient.New(baseURL)
	require.NoError(t, err)
	return c
}

func assertSentinel(t *testing.T, gotErr, wantTarget error) {
	t.Helper()
	require.Error(t, gotErr)
	assert.ErrorIs(t, gotErr, wantTarget)
}
