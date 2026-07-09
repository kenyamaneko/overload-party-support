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

func TestServer(t *testing.T) {
	t.Run("サーバフェイク", func(t *testing.T) {
		t.Run("Fn 未設定の endpoint は既定応答を返す", func(t *testing.T) {
			cases := []struct {
				name       string
				method     string
				path       string
				reqBody    []byte
				wantStatus int
			}{
				{
					name:       "ListAnnouncements (Fn 未設定) のとき、200 になる",
					method:     http.MethodGet,
					path:       "/api/v1/support/announcements?lang=ja",
					reqBody:    nil,
					wantStatus: http.StatusOK,
				},
				{
					name:       "GetAnnouncement (Fn 未設定) のとき、404 になる",
					method:     http.MethodGet,
					path:       "/api/v1/support/announcements/42?lang=ja",
					reqBody:    nil,
					wantStatus: http.StatusNotFound,
				},
				{
					name:       "SubmitInquiry (Fn 未設定) のとき、200 になる",
					method:     http.MethodPost,
					path:       "/api/v1/inquiries",
					reqBody:    []byte(`{"title":"t","body":"b","reply_email":"e@example.com"}`),
					wantStatus: http.StatusOK,
				},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					srv := apisupportserverfake.NewServer()
					defer srv.Close()

					req, _ := http.NewRequest(tc.method, srv.URL()+tc.path, bytes.NewReader(tc.reqBody))
					req.Header.Set("Content-Type", "application/json")
					resp, err := http.DefaultClient.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()

					assert.Equal(t, tc.wantStatus, resp.StatusCode)
				})
			}
		})

		t.Run("ListAnnouncementsFn は lang を受け取り wire レスポンスを返す", func(t *testing.T) {
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

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/api/v1/support/announcements?lang=ja", nil)
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
		})

		t.Run("GetAnnouncementFn は announcement_id を int64 で受け取る", func(t *testing.T) {
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

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/api/v1/support/announcements/77?lang=en", nil)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, int64(77), gotID)
			assert.Equal(t, "en", gotLang)
		})

		t.Run("不正な announcement_id のとき、Fn を呼ばず 404 を返す", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			called := false
			srv.GetAnnouncementFn = func(int64, string) (int, any) {
				called = true
				return http.StatusOK, nil
			}

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/api/v1/support/announcements/notnum?lang=ja", nil)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.False(t, called, "id parse 失敗時は Fn を呼ばない")
		})

		t.Run("SubmitInquiryFn は SubmitInquiryRequest を typed で受け取る", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			var gotReq apisupport.SubmitInquiryRequest
			srv.SubmitInquiryFn = func(req apisupport.SubmitInquiryRequest) (int, any) {
				gotReq = req
				return http.StatusOK, apisupport.SubmitInquiryResult{InquiryID: 999}
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

			var decoded apisupport.SubmitInquiryResult
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
			assert.Equal(t, int64(999), decoded.InquiryID)
		})
	})
}
