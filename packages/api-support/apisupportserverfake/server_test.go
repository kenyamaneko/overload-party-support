package apisupportserverfake_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
)

func TestServer(t *testing.T) {
	t.Run("サーバフェイク", func(t *testing.T) {
		t.Run("GetAnnouncement は Fn 未設定のとき、404 になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/api/v1/support/announcements/42?lang=ja", nil)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("ListAnnouncementsFn は lang を受け取る", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			var gotLang string
			srv.ListAnnouncementsFn = func(lang string) (int, any) {
				gotLang = lang
				return http.StatusOK, apisupport.AnnouncementListResponse{Announcements: []apisupport.AnnouncementSummary{}}
			}

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/api/v1/support/announcements?lang=ja", nil)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "ja", gotLang)
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
		})
	})
}
