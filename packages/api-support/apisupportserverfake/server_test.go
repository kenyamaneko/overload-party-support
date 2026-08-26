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
		t.Run("お知らせ一覧を返す処理を設定していないとき、ステータス200でannouncementsが空配列の本文を返す", func(t *testing.T) {
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

		t.Run("お知らせ一覧を返す処理を設定しているとき、その処理にlangクエリパラメータの値が渡る", func(t *testing.T) {
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

		t.Run("お知らせ一覧を返す処理を設定しているとき、その処理の戻り値のステータスと本文がそのまま応答になる", func(t *testing.T) {
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

		t.Run("お知らせ一覧を返す処理の戻り値の本文がnilのとき、応答本文は空になる", func(t *testing.T) {
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
		t.Run("announcementIdが数値としてパースできない値のとき、ステータス404を返す", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements/abc?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("announcementIdが数値としてパースできない値のとき、お知らせ詳細を返す処理を呼ばない", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()
			called := false
			srv.GetAnnouncementFn = func(announcementID int64, lang string) (int, any) {
				called = true
				return http.StatusOK, nil
			}

			_, err := http.Get(srv.URL() + "/api/v1/support/announcements/abc?lang=ja")
			require.NoError(t, err)

			assert.False(t, called)
		})

		t.Run("お知らせ詳細を返す処理を設定していないとき、ステータス404を返す", func(t *testing.T) {
			srv := apisupportserverfake.NewServer()
			defer srv.Close()

			resp, err := http.Get(srv.URL() + "/api/v1/support/announcements/1?lang=ja")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("お知らせ詳細を返す処理を設定しているとき、その処理にint64へパースしたannouncementIdの値とlangクエリパラメータの値が渡る", func(t *testing.T) {
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

		t.Run("お知らせ詳細を返す処理を設定しているとき、その処理の戻り値のステータスと本文がそのまま応答になる", func(t *testing.T) {
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

		t.Run("お知らせ詳細を返す処理の戻り値の本文がnilのとき、応答本文は空になる", func(t *testing.T) {
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
