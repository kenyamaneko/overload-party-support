package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/service/inquiry"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

func newInquiryEngine(h *rest.InquiryHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/internal/v1/inquiries", h.List)
	r.GET("/internal/v1/inquiries/:inquiryId", h.GetDetail)
	r.POST("/internal/v1/inquiries/:inquiryId/status", h.UpdateStatus)
	r.POST("/internal/v1/inquiries/:inquiryId/note", h.UpdateNote)
	return r
}

// 仕様 (FEATURE_SPEC §8.3): status クエリの許容値以外は 400。
func TestInquiryList_仕様_statusバリデーション(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "status 未指定は全件で 200",
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "new のみ",
			query:      "?status=new",
			wantStatus: http.StatusOK,
		},
		{
			name:       "new,in_progress",
			query:      "?status=new,in_progress",
			wantStatus: http.StatusOK,
		},
		{
			name:       "未知値は 400",
			query:      "?status=invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &port.MockInquiryStore{
				ListFn: func(_ context.Context, _ []apisupport.Status) ([]apisupport.Inquiry, error) {
					return nil, nil
				},
			}
			h := rest.NewInquiryHandler(inquiry.New(store, nil, nil, 200))

			req := httptest.NewRequest(http.MethodGet, "/internal/v1/inquiries"+tc.query, nil)
			w := httptest.NewRecorder()
			newInquiryEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.1 / API_REFERENCE): UpdateStatus は遷移表に従う。
// - 未知値 → 400, 非存在 → 404, 逆遷移 → 409。
func TestInquiryUpdateStatus_仕様_HTTPマッピング(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		current    apisupport.Status
		wantStatus int
	}{
		{
			name:       "new → in_progress は 200",
			body:       `{"status":"in_progress"}`,
			current:    apisupport.StatusNew,
			wantStatus: http.StatusOK,
		},
		{
			name:       "new → closed は 200",
			body:       `{"status":"closed"}`,
			current:    apisupport.StatusNew,
			wantStatus: http.StatusOK,
		},
		{
			name:       "未知値は 400",
			body:       `{"status":"unknown"}`,
			current:    apisupport.StatusNew,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "new 指定は 409 (逆遷移扱い)",
			body:       `{"status":"new"}`,
			current:    apisupport.StatusNew,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "closed → in_progress は 409",
			body:       `{"status":"in_progress"}`,
			current:    apisupport.StatusClosed,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "不正 JSON は 400",
			body:       `{bad}`,
			current:    apisupport.StatusNew,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &port.MockInquiryStore{
				GetFn: func(_ context.Context, _ int64) (*apisupport.Inquiry, error) {
					return &apisupport.Inquiry{InquiryID: 1, Status: tc.current, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
				},
				UpdateStatusFn: func(_ context.Context, _ int64, s apisupport.Status) (*apisupport.Inquiry, error) {
					return &apisupport.Inquiry{InquiryID: 1, Status: s, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
				},
			}
			h := rest.NewInquiryHandler(inquiry.New(store, nil, nil, 200))

			req := httptest.NewRequest(http.MethodPost, "/internal/v1/inquiries/1/status", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			newInquiryEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

// 仕様 (API_REFERENCE): GetDetail は非存在で 404、非数値 ID も 404、不正リクエストパスの扱いを HTTP に射影する。
func TestInquiryGetDetail_仕様_HTTPマッピング(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		getErr     error
		wantStatus int
	}{
		{
			name:       "正常",
			id:         "1",
			getErr:     nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "非数値 ID は 404",
			id:         "abc",
			getErr:     nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "存在しない ID は 404",
			id:         "999",
			getErr:     port.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &port.MockInquiryStore{
				GetFn: func(_ context.Context, _ int64) (*apisupport.Inquiry, error) {
					if tc.getErr != nil {
						return nil, tc.getErr
					}
					return &apisupport.Inquiry{InquiryID: 1, Status: apisupport.StatusNew, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
				},
			}
			h := rest.NewInquiryHandler(inquiry.New(store, nil, nil, 200))

			req := httptest.NewRequest(http.MethodGet, "/internal/v1/inquiries/"+tc.id, nil)
			w := httptest.NewRecorder()
			newInquiryEngine(h).ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

// 仕様 (FEATURE_SPEC §8.3): List は updated_at DESC 順の結果を JSON 配列で返す。0 件でも nil でない。
func TestInquiryList_仕様_空配列レスポンス(t *testing.T) {
	store := &port.MockInquiryStore{
		ListFn: func(_ context.Context, _ []apisupport.Status) ([]apisupport.Inquiry, error) {
			return nil, nil
		},
	}
	h := rest.NewInquiryHandler(inquiry.New(store, nil, nil, 200))

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/inquiries", nil)
	w := httptest.NewRecorder()
	newInquiryEngine(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apisupport.InquiryListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Inquiries)
	assert.Empty(t, resp.Inquiries)
}

// 仕様: UpdateNote は nil / 空文字でメモ削除扱い。JSON レスポンスに internal_note が正しく載る。
func TestInquiryUpdateNote_仕様_レスポンス(t *testing.T) {
	note := "running"
	store := &port.MockInquiryStore{
		UpdateNoteFn: func(_ context.Context, _ int64, got *string) (*apisupport.Inquiry, error) {
			return &apisupport.Inquiry{InquiryID: 1, Status: apisupport.StatusNew, InternalNote: got, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
	}
	h := rest.NewInquiryHandler(inquiry.New(store, nil, nil, 200))

	body, _ := json.Marshal(apisupport.UpdateInquiryNoteRequest{InternalNote: &note})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/inquiries/1/note", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newInquiryEngine(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apisupport.InquiryDetail
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.InternalNote)
	assert.Equal(t, note, *resp.InternalNote)
}
