package apisupportclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportclient"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 以下の TestClient_<Endpoint>_StatusMapping 群は、SDK の固有責務である
// 「OpenAPI spec で宣言された 4xx/5xx status を sentinel error に変換する」契約を
// endpoint ごとに検証する。各テスト関数は data/openapi.yaml が宣言する error status を
// 網羅し、errors.Is で意図した sentinel に一致することを確認する。

func TestClient_ListAnnouncements_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apisupportclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(_ string) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListAnnouncements(context.Background(), "ja")
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_GetAnnouncement_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apisupportclient.ErrBadRequest,
		},
		{
			name:       "404 を受けたとき ErrNotFound",
			status:     http.StatusNotFound,
			wantTarget: apisupportclient.ErrNotFound,
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
}

func TestClient_SubmitInquiry_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apisupportclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.SubmitInquiryFn = func(_ apisupport.SubmitInquiryRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			// status mapping 検証のため request body の内容は無関係 (server fake は body を見ず tc.status を返す)。
			_, err := c.SubmitInquiry(context.Background(), apisupport.SubmitInquiryRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

// TestClient_RequestEditor は Option pattern の契約 (WithRequestEditorFn で渡した
// editor が全リクエストに適用される) を検証する。header 注入の接続点として SDK が
// 機能することを担保する。
func TestClient_RequestEditor(t *testing.T) {
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
