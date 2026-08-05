package external_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/handler/external"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

func newExternalEngine(h *external.InquiryHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/inquiries", h.Submit)
	return r
}

func TestSubmit(t *testing.T) {
	t.Run("問い合わせ送信API", func(t *testing.T) {
		cases := []struct {
			name       string
			body       string
			slackErr   error
			wantStatus int
		}{
			{
				name:       "正常なbodyのとき、200になる",
				body:       `{"title":"T","body":"B","reply_email":"u@e.com"}`,
				wantStatus: http.StatusOK,
			},
			{
				name:       "titleが欠落するとき、400になる",
				body:       `{"body":"B","reply_email":"u@e.com"}`,
				wantStatus: http.StatusBadRequest,
			},
			{
				name:       "reply_emailが形式不正のとき、400になる",
				body:       `{"title":"T","body":"B","reply_email":"not-email"}`,
				wantStatus: http.StatusBadRequest,
			},
			{
				name:       "不正なJSONのとき、400になる",
				body:       `{bad`,
				wantStatus: http.StatusBadRequest,
			},
			{
				name:       "Slack通知が失敗するとき、500になる",
				body:       `{"title":"T","body":"B","reply_email":"u@e.com"}`,
				slackErr:   errors.New("slack"),
				wantStatus: http.StatusInternalServerError,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				store := &port.MockInquiryStore{
					CreateFn: func(_ context.Context, title, body, replyEmail string) (*domain.Inquiry, error) {
						return &domain.Inquiry{InquiryID: 1, Title: title, Body: body, ReplyEmail: replyEmail, Status: "new", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
					},
				}
				slack := &port.MockSlackNotifier{
					NotifyInquiryReceivedFn: func(_ context.Context, _ *domain.Inquiry, _ string) error {
						return tc.slackErr
					},
				}
				email := &port.MockEmailSender{
					SendInquiryReceiptFn: func(_ context.Context, _ *domain.Inquiry, _ string) error { return nil },
				}
				h := external.NewInquiryHandler(inquiry.New(store, slack, email, 200))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/inquiries", bytes.NewReader([]byte(tc.body)))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				newExternalEngine(h).ServeHTTP(w, req)

				assert.Equal(t, tc.wantStatus, w.Code)
			})
		}

		t.Run("採番されたinquiry_idがレスポンスに含まれる", func(t *testing.T) {
			store := &port.MockInquiryStore{
				CreateFn: func(_ context.Context, title, body, replyEmail string) (*domain.Inquiry, error) {
					return &domain.Inquiry{InquiryID: 42, Title: title, Body: body, ReplyEmail: replyEmail, Status: "new", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
				},
			}
			slack := &port.MockSlackNotifier{
				NotifyInquiryReceivedFn: func(_ context.Context, _ *domain.Inquiry, _ string) error { return nil },
			}
			email := &port.MockEmailSender{
				SendInquiryReceiptFn: func(_ context.Context, _ *domain.Inquiry, _ string) error { return nil },
			}
			h := external.NewInquiryHandler(inquiry.New(store, slack, email, 200))

			body, _ := json.Marshal(apisupport.SubmitInquiryRequest{Title: "T", Body: "B", ReplyEmail: "u@e.com"})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/inquiries", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			newExternalEngine(h).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var resp apisupport.SubmitInquiryResult
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, int64(42), resp.InquiryID)
		})
	})
}
