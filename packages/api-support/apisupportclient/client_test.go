package apisupportclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/packages/api-support/apisupportclient"
)

// newStatusServer は常に status を返し、body が nil でなければ JSON として書き込むテスト用サーバを返す。
func newStatusServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body != nil {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if body != nil {
			require.NoError(t, json.NewEncoder(w).Encode(body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClientStatusErrorConversion(t *testing.T) {
	t.Run("[公開クライアントライブラリ]ステータスコードからのエラー変換", func(t *testing.T) {
		t.Run("応答が400のとき、apisupportclient.ErrBadRequestを返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusBadRequest, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			assert.ErrorIs(t, err, apisupportclient.ErrBadRequest)
		})

		t.Run("応答が401のとき、apisupportclient.ErrUnauthorizedを返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusUnauthorized, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			assert.ErrorIs(t, err, apisupportclient.ErrUnauthorized)
		})

		t.Run("応答が404のとき、apisupportclient.ErrNotFoundを返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusNotFound, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			assert.ErrorIs(t, err, apisupportclient.ErrNotFound)
		})

		t.Run("応答が500以上のとき、apisupportclient.ErrInternalServerを返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusInternalServerError, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			assert.ErrorIs(t, err, apisupportclient.ErrInternalServer)
		})

		t.Run("応答が上記のいずれの区分にも該当しない300のとき、いずれのsentinelでもない、呼び出し元の操作名とステータスコードを含むエラーを返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusMultipleChoices, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			require.Error(t, err)
			assert.NotErrorIs(t, err, apisupportclient.ErrBadRequest)
			assert.NotErrorIs(t, err, apisupportclient.ErrUnauthorized)
			assert.NotErrorIs(t, err, apisupportclient.ErrNotFound)
			assert.NotErrorIs(t, err, apisupportclient.ErrInternalServer)
			assert.Contains(t, err.Error(), "ListAnnouncements")
			assert.Contains(t, err.Error(), "300")
		})
	})
}

func TestClientGetHealth(t *testing.T) {
	t.Run("[公開クライアントライブラリ]GetHealth", func(t *testing.T) {
		t.Run("応答が200のとき、応答本文をそのまま返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusOK, map[string]string{"status": "ok"})
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			got, err := c.GetHealth(context.Background())

			require.NoError(t, err)
			assert.Equal(t, "ok", got.Status)
		})

		t.Run("応答が200以外のとき、ステータスコードからのエラー変換の規則に従ったエラーを返し、エラーメッセージに操作名GetHealthが含まれる", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusInternalServerError, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.GetHealth(context.Background())

			assert.ErrorIs(t, err, apisupportclient.ErrInternalServer)
			assert.Contains(t, err.Error(), "GetHealth")
		})
	})
}

func TestClientListAnnouncements(t *testing.T) {
	t.Run("[公開クライアントライブラリ]ListAnnouncements", func(t *testing.T) {
		t.Run("呼び出し時、指定したlangをクエリパラメータとして送信する", func(t *testing.T) {
			var gotLang string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotLang = r.URL.Query().Get("lang")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"announcements": []any{}}))
			}))
			defer srv.Close()
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "en")

			require.NoError(t, err)
			assert.Equal(t, "en", gotLang)
		})

		t.Run("応答が200のとき、応答本文をそのまま返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusOK, map[string]any{
				"announcements": []map[string]any{
					{"announcement_id": 1, "type": "info", "title": "タイトル", "published_at": "2026-01-01T00:00:00Z"},
				},
			})
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			got, err := c.ListAnnouncements(context.Background(), "ja")

			require.NoError(t, err)
			require.Len(t, got.Announcements, 1)
			assert.Equal(t, int64(1), got.Announcements[0].AnnouncementID)
			assert.Equal(t, "タイトル", got.Announcements[0].Title)
		})

		t.Run("応答が200以外のとき、ステータスコードからのエラー変換の規則に従ったエラーを返し、エラーメッセージに操作名ListAnnouncementsが含まれる", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusNotFound, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			assert.ErrorIs(t, err, apisupportclient.ErrNotFound)
			assert.Contains(t, err.Error(), "ListAnnouncements")
		})
	})
}

func TestClientGetAnnouncement(t *testing.T) {
	t.Run("[公開クライアントライブラリ]GetAnnouncement", func(t *testing.T) {
		t.Run("呼び出し時、指定したannouncementIDとlangをリクエストとして送信する", func(t *testing.T) {
			var gotPath, gotLang string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotLang = r.URL.Query().Get("lang")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"announcement_id": 42, "type": "info", "title": "t", "body": "b", "published_at": nil,
				}))
			}))
			defer srv.Close()
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.GetAnnouncement(context.Background(), 42, "en")

			require.NoError(t, err)
			assert.Contains(t, gotPath, "42")
			assert.Equal(t, "en", gotLang)
		})

		t.Run("応答が200のとき、応答本文をそのまま返す", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusOK, map[string]any{
				"announcement_id": 7, "type": "event", "title": "タイトル", "body": "本文", "published_at": nil,
			})
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			got, err := c.GetAnnouncement(context.Background(), 7, "ja")

			require.NoError(t, err)
			assert.Equal(t, int64(7), got.AnnouncementID)
			assert.Equal(t, "タイトル", got.Title)
			assert.Equal(t, "本文", got.Body)
		})

		t.Run("応答が200以外のとき、ステータスコードからのエラー変換の規則に従ったエラーを返し、エラーメッセージに操作名GetAnnouncementが含まれる", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusBadRequest, nil)
			c, err := apisupportclient.New(srv.URL)
			require.NoError(t, err)

			_, err = c.GetAnnouncement(context.Background(), 1, "ja")

			assert.ErrorIs(t, err, apisupportclient.ErrBadRequest)
			assert.Contains(t, err.Error(), "GetAnnouncement")
		})
	})
}

func TestClientRequestOptions(t *testing.T) {
	t.Run("[公開クライアントライブラリ]リクエストオプション", func(t *testing.T) {
		t.Run("WithHTTPClientで差し替えたHTTPクライアントを指定したとき、そのクライアントを介してリクエストが送信される", func(t *testing.T) {
			srv := newStatusServer(t, http.StatusOK, map[string]any{"announcements": []any{}})
			recorder := &recordingDoer{base: http.DefaultClient}
			c, err := apisupportclient.New(srv.URL, apisupportclient.WithHTTPClient(recorder))
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")

			require.NoError(t, err)
			assert.Equal(t, 1, recorder.calls)
		})

		t.Run("WithRequestEditorFnで設定したリクエスト編集処理は、送信する全てのリクエストに適用される", func(t *testing.T) {
			const headerName = "X-Test-Editor"
			var gotHeaders []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeaders = append(gotHeaders, r.Header.Get(headerName))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				switch {
				case r.URL.Path == "/api/v1/support/announcements":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"announcements": []any{}}))
				default:
					require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"status": "ok"}))
				}
			}))
			defer srv.Close()
			c, err := apisupportclient.New(srv.URL, apisupportclient.WithRequestEditorFn(
				func(ctx context.Context, req *http.Request) error {
					req.Header.Set(headerName, "applied")
					return nil
				},
			))
			require.NoError(t, err)

			_, err = c.ListAnnouncements(context.Background(), "ja")
			require.NoError(t, err)
			_, err = c.GetHealth(context.Background())
			require.NoError(t, err)

			require.Len(t, gotHeaders, 2)
			assert.Equal(t, "applied", gotHeaders[0])
			assert.Equal(t, "applied", gotHeaders[1])
		})
	})
}

// recordingDoer は apisupport.HttpRequestDoer を満たし、呼び出し回数を数える。
type recordingDoer struct {
	base  *http.Client
	calls int
}

func (d *recordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.calls++
	return d.base.Do(req)
}
