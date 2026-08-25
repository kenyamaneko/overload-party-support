package apisupportserverfake_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportserverfake"
)

func TestServerListAnnouncements(t *testing.T) {
	t.Run("[テスト用フェイクサーバ]お知らせ一覧エンドポイント", func(t *testing.T) {
		t.Run("ListAnnouncementsFnが未設定のとき、ステータス200でannouncementsが空配列の本文を返す", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.JSONEq(t, `{"announcements":[]}`, string(body))
		})

		t.Run("ListAnnouncementsFnが設定されているとき、そのFnにlangクエリパラメータの値が渡る", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			var gotLang string
			srv.ListAnnouncementsFn = func(lang string) (int, any) {
				gotLang = lang
				return http.StatusOK, nil
			}

			_, err := http.Get(srv.URL() + "/api/v1/support/announcements?lang=en")
			require.NoError(t, err)

			assert.Equal(t, "en", gotLang)
		})

		t.Run("ListAnnouncementsFnが設定されているとき、その戻り値のステータスと本文がそのまま応答になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(lang string) (int, any) {
				return http.StatusTeapot, map[string]string{"custom": "value"}
			}

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, http.StatusTeapot, resp.StatusCode)
			assert.JSONEq(t, `{"custom":"value"}`, string(body))
		})

		t.Run("ListAnnouncementsFnの戻り値の本文がnilのとき、応答本文は空になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.ListAnnouncementsFn = func(lang string) (int, any) {
				return http.StatusNoContent, nil
			}

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Empty(t, body)
		})
	})
}

func TestServerGetAnnouncement(t *testing.T) {
	t.Run("[テスト用フェイクサーバ]お知らせ詳細エンドポイント", func(t *testing.T) {
		t.Run("announcementIdが数値としてパースできない値のとき、GetAnnouncementFnを呼ばずステータス404を返す", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			called := false
			srv.GetAnnouncementFn = func(announcementID int64, lang string) (int, any) {
				called = true
				return http.StatusOK, nil
			}

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements/abc?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.False(t, called)
		})

		t.Run("GetAnnouncementFnが未設定のとき、ステータス404を返す", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements/1?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GetAnnouncementFnが設定されているとき、そのFnにint64にパースした値のannouncementIdとlangクエリパラメータの値が渡る", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			var gotID int64
			var gotLang string
			srv.GetAnnouncementFn = func(announcementID int64, lang string) (int, any) {
				gotID = announcementID
				gotLang = lang
				return http.StatusOK, nil
			}

			_, err := http.Get(srv.URL() + "/api/v1/support/announcements/42?lang=en")
			require.NoError(t, err)

			assert.Equal(t, int64(42), gotID)
			assert.Equal(t, "en", gotLang)
		})

		t.Run("GetAnnouncementFnが設定されているとき、その戻り値のステータスと本文がそのまま応答になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.GetAnnouncementFn = func(announcementID int64, lang string) (int, any) {
				return http.StatusTeapot, map[string]string{"custom": "value"}
			}

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements/1?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, http.StatusTeapot, resp.StatusCode)
			assert.JSONEq(t, `{"custom":"value"}`, string(body))
		})

		t.Run("GetAnnouncementFnの戻り値の本文がnilのとき、応答本文は空になる", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			srv.GetAnnouncementFn = func(announcementID int64, lang string) (int, any) {
				return http.StatusNoContent, nil
			}

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements/1?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Empty(t, body)
		})
	})
}
