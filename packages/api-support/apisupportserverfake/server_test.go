package apisupportserverfake_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
)

func TestServer(t *testing.T) {
	t.Run("サーバフェイク", func(t *testing.T) {
		t.Run("GetAnnouncementはFn未設定のとき、404になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/api/v1/support/announcements/42?lang=ja", nil)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("ListAnnouncementsFnはlangを受け取る", func(t *testing.T) {
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

		t.Run("GetAnnouncementFnはannouncement_idをint64で受け取る", func(t *testing.T) {
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

		t.Run("不正なannouncement_idのとき、Fnを呼ばず404を返す", func(t *testing.T) {
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
	})
}
