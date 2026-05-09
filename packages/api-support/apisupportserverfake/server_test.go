package apisupportserverfake_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
)

// Fn 未設定の endpoint は既定応答を返す。
func TestServer_DefaultResponses(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		reqBody    []byte
		wantStatus int
	}{
		{
			name:       "ListAnnouncements 既定は 200 + 空配列",
			method:     http.MethodGet,
			path:       "/internal/v1/announcements?lang=ja",
			reqBody:    nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "GetAnnouncement (Fn 未設定) 既定は 404",
			method:     http.MethodGet,
			path:       "/internal/v1/announcements/42?lang=ja",
			reqBody:    nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "SubmitInquiry 既定は 200 + 空 Response",
			method:     http.MethodPost,
			path:       "/api/v1/inquiries",
			reqBody:    []byte(`{"title":"t","body":"b","reply_email":"e@example.com"}`),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			req, _ := http.NewRequest(tt.method, srv.URL()+tt.path, bytes.NewReader(tt.reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// ListAnnouncementsFn は lang を typed で受け取り wire レスポンスを返せる。
func TestServer_ListAnnouncementsFn(t *testing.T) {
	srv := apisupportserverfake.NewServer()
	defer srv.Close()

	pub := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	var gotLang string
	srv.ListAnnouncementsFn = func(lang string) (int, any) {
		gotLang = lang
		return http.StatusOK, apisupport.AnnouncementListResponse{
			Announcements: []apisupport.AnnouncementSummary{
				{
					AnnouncementID: 1,
					Type:           apisupport.AnnouncementTypeInfo,
					Title:          "T",
					PublishedAt:    pub,
				},
			},
		}
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/internal/v1/announcements?lang=ja", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ja", gotLang)

	var decoded apisupport.AnnouncementListResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	require.Len(t, decoded.Announcements, 1)
	assert.Equal(t, int64(1), decoded.Announcements[0].AnnouncementID)
	assert.Equal(t, apisupport.AnnouncementTypeInfo, decoded.Announcements[0].Type)
}

// GetAnnouncementFn は path の announcement_id を int64 で受け取れる。
func TestServer_GetAnnouncementFn_ReceivesTypedID(t *testing.T) {
	srv := apisupportserverfake.NewServer()
	defer srv.Close()

	var gotID int64
	var gotLang string
	srv.GetAnnouncementFn = func(announcementID int64, lang string) (int, any) {
		gotID = announcementID
		gotLang = lang
		return http.StatusOK, apisupport.AnnouncementDetail{
			AnnouncementID: announcementID,
			Type:           apisupport.AnnouncementTypeMaintenance,
			Title:          "T",
			Body:           "B",
			PublishedAt:    nil,
		}
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/internal/v1/announcements/77?lang=en", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int64(77), gotID)
	assert.Equal(t, "en", gotLang)
}

// 不正な announcement_id は Fn を呼ばず 404 を返す。
func TestServer_GetAnnouncement_InvalidIDReturns404(t *testing.T) {
	srv := apisupportserverfake.NewServer()
	defer srv.Close()

	called := false
	srv.GetAnnouncementFn = func(int64, string) (int, any) {
		called = true
		return http.StatusOK, nil
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/internal/v1/announcements/notnum?lang=ja", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.False(t, called, "id parse 失敗時は Fn を呼ばない")
}

// SubmitInquiryFn は apisupport.SubmitInquiryRequest を typed で受け取れる。
func TestServer_SubmitInquiryFn_ReceivesTypedRequest(t *testing.T) {
	srv := apisupportserverfake.NewServer()
	defer srv.Close()

	var gotReq apisupport.SubmitInquiryRequest
	srv.SubmitInquiryFn = func(req apisupport.SubmitInquiryRequest) (int, any) {
		gotReq = req
		return http.StatusOK, apisupport.SubmitInquiryResponse{InquiryID: 999}
	}

	reqBody := []byte(`{"title":"t","body":"b","reply_email":"e@example.com"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/api/v1/inquiries", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "t", gotReq.Title)
	assert.Equal(t, "b", gotReq.Body)
	assert.Equal(t, "e@example.com", gotReq.ReplyEmail)

	var decoded apisupport.SubmitInquiryResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	assert.Equal(t, int64(999), decoded.InquiryID)
}
