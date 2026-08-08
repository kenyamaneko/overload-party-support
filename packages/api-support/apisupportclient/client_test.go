package apisupportclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportclient"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListAnnouncements(t *testing.T) {
	t.Run("ListAnnouncements", func(t *testing.T) {
		t.Run("400を受けたとき、ErrBadRequestになる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(_ string) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListAnnouncements(context.Background(), "ja")
			assertSentinel(t, err, apisupportclient.ErrBadRequest)
		})

		t.Run("401を受けたとき、ErrUnauthorizedになる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(_ string) (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListAnnouncements(context.Background(), "ja")
			assertSentinel(t, err, apisupportclient.ErrUnauthorized)
		})

		t.Run("400を受けたとき、エラーメッセージに操作名ListAnnouncementsが含まれる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(_ string) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListAnnouncements(context.Background(), "ja")
			assert.ErrorContains(t, err, "apisupportclient: ListAnnouncements:")
		})
	})
}

func TestClient_GetAnnouncement(t *testing.T) {
	t.Run("GetAnnouncement", func(t *testing.T) {
		cases := []struct {
			name       string
			status     int
			wantTarget error
		}{
			{
				name:       "400を受けたとき、ErrBadRequestになる",
				status:     http.StatusBadRequest,
				wantTarget: apisupportclient.ErrBadRequest,
			},
			{
				name:       "404を受けたとき、ErrNotFoundになる",
				status:     http.StatusNotFound,
				wantTarget: apisupportclient.ErrNotFound,
			},
			{
				name:       "500を受けたとき、ErrInternalServerになる",
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

		t.Run("404を受けたとき、エラーメッセージに操作名GetAnnouncementが含まれる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.GetAnnouncementFn = func(_ int64, _ string) (int, any) { return http.StatusNotFound, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetAnnouncement(context.Background(), 1, "ja")
			assert.ErrorContains(t, err, "apisupportclient: GetAnnouncement:")
		})
	})
}

func TestClient_RequestEditor(t *testing.T) {
	t.Run("リクエストエディタの適用", func(t *testing.T) {
		t.Run("設定したヘッダが送信先の全リクエストに付与される", func(t *testing.T) {
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
