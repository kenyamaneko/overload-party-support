package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-support/internal/router"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/announcement"
)

var errFromQuerier = errors.New("querier: boom")

// seedPublishedAnnouncement は実DBに公開中のお知らせを1件登録し、announcement_idを返す。
func seedPublishedAnnouncement(t *testing.T, lang, title, body string) int64 {
	t.Helper()
	publishedAt := time.Now().Add(-1 * time.Hour)

	var id int64
	err := pg.Pool.QueryRow(context.Background(),
		`INSERT INTO support.announcements (type, published_at) VALUES ($1, $2) RETURNING announcement_id`,
		domain.TypeInfo, publishedAt,
	).Scan(&id)
	require.NoError(t, err)

	_, err = pg.Pool.Exec(context.Background(),
		`INSERT INTO support.announcement_translations (announcement_id, lang, title, body) VALUES ($1, $2, $3, $4)`,
		id, lang, title, body,
	)
	require.NoError(t, err)

	return id
}

func TestList(t *testing.T) {
	t.Run("[公開お知らせAPI]お知らせ一覧の取得", func(t *testing.T) {
		t.Run("langクエリパラメータが無いとき、ステータス400と本文{\"error\":\"announcement: lang is required\"}を返す", func(t *testing.T) {
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(&port.MockAnnouncementRepo{}, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code)
			assert.JSONEq(t, `{"error":"announcement: lang is required"}`, w.Body.String())
		})

		t.Run("langクエリパラメータが対応外の値のとき、ステータス400と本文{\"error\":\"announcement: unsupported lang\"}を返す", func(t *testing.T) {
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(&port.MockAnnouncementRepo{}, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements?lang=fr", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code)
			assert.JSONEq(t, `{"error":"announcement: unsupported lang"}`, w.Body.String())
		})

		t.Run("お知らせ取得ポートが想定外のエラーを返すとき、ステータス500を返す", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				ListPublishedFn: func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
					return nil, errFromQuerier
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements?lang=ja", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 500, w.Code)
		})

		t.Run("お知らせ取得ポートが結果としてnil(該当なし)を返すとき、応答本文のannouncementsはnullでなく空配列になる", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				ListPublishedFn: func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
					return nil, nil
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements?lang=ja", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 200, w.Code)
			assert.JSONEq(t, `{"announcements":[]}`, w.Body.String())
		})

		t.Run("取得結果が複数件あるとき、応答本文のannouncementsの各要素にannouncement_id・type・title・published_atが取得結果のとおり反映される", func(t *testing.T) {
			publishedAt1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			publishedAt2 := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
			want := []domain.AnnouncementSummary{
				{AnnouncementID: 1, Type: domain.TypeInfo, Title: "お知らせ1", PublishedAt: publishedAt1},
				{AnnouncementID: 2, Type: domain.TypeEvent, Title: "お知らせ2", PublishedAt: publishedAt2},
			}
			querier := &port.MockAnnouncementRepo{
				ListPublishedFn: func(ctx context.Context, lang string, now time.Time) ([]domain.AnnouncementSummary, error) {
					return want, nil
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements?lang=ja", nil)
			r.ServeHTTP(w, req)

			require.Equal(t, 200, w.Code)
			var body apisupport.AnnouncementListResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Len(t, body.Announcements, 2)
			assert.Equal(t, int64(1), body.Announcements[0].AnnouncementID)
			assert.Equal(t, apisupport.AnnouncementTypeInfo, body.Announcements[0].Type)
			assert.Equal(t, "お知らせ1", body.Announcements[0].Title)
			assert.True(t, publishedAt1.Equal(body.Announcements[0].PublishedAt))
			assert.Equal(t, int64(2), body.Announcements[1].AnnouncementID)
			assert.Equal(t, apisupport.AnnouncementTypeEvent, body.Announcements[1].Type)
			assert.Equal(t, "お知らせ2", body.Announcements[1].Title)
			assert.True(t, publishedAt2.Equal(body.Announcements[1].PublishedAt))
		})
	})
}

func TestGetDetail(t *testing.T) {
	t.Run("[公開お知らせAPI]お知らせ詳細の取得", func(t *testing.T) {
		t.Run("announcementIdが数値としてパースできない値のとき、ステータス404と本文{\"error\":\"announcement not found\"}を返す", func(t *testing.T) {
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(&port.MockAnnouncementRepo{}, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/abc?lang=ja", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 404, w.Code)
			assert.JSONEq(t, `{"error":"announcement not found"}`, w.Body.String())
		})

		t.Run("langクエリパラメータが無いとき、ステータス400と本文{\"error\":\"announcement: lang is required\"}を返す", func(t *testing.T) {
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(&port.MockAnnouncementRepo{}, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/1", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code)
			assert.JSONEq(t, `{"error":"announcement: lang is required"}`, w.Body.String())
		})

		t.Run("langクエリパラメータが対応外の値のとき、ステータス400と本文{\"error\":\"announcement: unsupported lang\"}を返す", func(t *testing.T) {
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(&port.MockAnnouncementRepo{}, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/1?lang=fr", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code)
			assert.JSONEq(t, `{"error":"announcement: unsupported lang"}`, w.Body.String())
		})

		t.Run("お知らせ取得ポートがport.ErrNotFoundを返すとき、ステータス404と本文{\"error\":\"announcement: not found\"}を返す", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return nil, port.ErrNotFound
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/1?lang=ja", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 404, w.Code)
			assert.JSONEq(t, `{"error":"announcement: not found"}`, w.Body.String())
		})

		t.Run("お知らせ取得ポートが想定外のエラーを返すとき、ステータス500を返す", func(t *testing.T) {
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return nil, errFromQuerier
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/1?lang=ja", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 500, w.Code)
		})

		t.Run("お知らせが取得できたとき、応答本文にannouncement_id・type・title・body・published_atが取得結果のとおり反映される", func(t *testing.T) {
			publishedAt := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
			want := &domain.AnnouncementDetail{
				AnnouncementID: 7,
				Type:           domain.TypeMaintenance,
				Title:          "メンテのお知らせ",
				Body:           "本文です",
				PublishedAt:    &publishedAt,
			}
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return want, nil
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/7?lang=ja", nil)
			r.ServeHTTP(w, req)

			require.Equal(t, 200, w.Code)
			var body apisupport.AnnouncementDetail
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, int64(7), body.AnnouncementID)
			assert.Equal(t, apisupport.AnnouncementTypeMaintenance, body.Type)
			assert.Equal(t, "メンテのお知らせ", body.Title)
			assert.Equal(t, "本文です", body.Body)
			require.NotNil(t, body.PublishedAt)
			assert.True(t, publishedAt.Equal(*body.PublishedAt))
		})

		t.Run("取得結果のpublished_atが未設定のとき、応答本文のpublished_atはnullになる", func(t *testing.T) {
			want := &domain.AnnouncementDetail{
				AnnouncementID: 8,
				Type:           domain.TypeInfo,
				Title:          "下書き",
				Body:           "本文です",
				PublishedAt:    nil,
			}
			querier := &port.MockAnnouncementRepo{
				GetPublishedDetailFn: func(ctx context.Context, announcementID int64, lang string) (*domain.AnnouncementDetail, error) {
					return want, nil
				},
			}
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(querier, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/8?lang=ja", nil)
			r.ServeHTTP(w, req)

			require.Equal(t, 200, w.Code)
			var body apisupport.AnnouncementDetail
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Nil(t, body.PublishedAt)
		})
	})
}

func TestAnnouncementEndToEnd(t *testing.T) {
	t.Run("[公開お知らせAPI]エンドツーエンドの配線確認", func(t *testing.T) {
		t.Run("実DBに公開条件を満たすお知らせを登録した状態でそのannouncementIdを指定すると、ステータス200でその内容が返る", func(t *testing.T) {
			pg.Truncate(t)
			id := seedPublishedAnnouncement(t, "ja", "実DBのお知らせ", "実DBの本文")
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(repo, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/support/announcements/%d?lang=ja", id), nil)
			r.ServeHTTP(w, req)

			require.Equal(t, 200, w.Code)
			var body apisupport.AnnouncementDetail
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, id, body.AnnouncementID)
			assert.Equal(t, "実DBのお知らせ", body.Title)
			assert.Equal(t, "実DBの本文", body.Body)
		})

		t.Run("実DBに存在しないannouncementIdを指定すると、ステータス404が返る", func(t *testing.T) {
			pg.Truncate(t)
			repo := postgres.NewAnnouncementRepository(pg.Pool)
			r := router.NewInternal(rest.NewAnnouncementHandler(announcement.New(repo, time.Now)))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/support/announcements/1?lang=ja", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, 404, w.Code)
		})
	})
}
