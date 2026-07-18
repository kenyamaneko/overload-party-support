package sendgrid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

const (
	dummyAPIKey      = "SG-TST-0001"
	dummyFromAddress = "support@example.com"
	dummyFromName    = "Overload Party Support"

	dummyInquiryID int64 = 5
)

// rewriteHostTransport は SendGrid API 宛のリクエストを httptest.Server の host へ差し替える RoundTripper。
type rewriteHostTransport struct {
	target *url.URL
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// newTestSender は SendGrid API 応答先を httptest.Server に差し替えた RealSender を組み立てる。
func newTestSender(t *testing.T, serverURL string) *RealSender {
	t.Helper()
	target, err := url.Parse(serverURL)
	require.NoError(t, err)
	s, err := NewRealSender(dummyAPIKey, dummyFromAddress, dummyFromName)
	require.NoError(t, err)
	s.client = &http.Client{Transport: &rewriteHostTransport{target: target}}
	return s
}

func newInquiry(id int64) *domain.Inquiry {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	return &domain.Inquiry{
		InquiryID:  id,
		Title:      "T",
		Body:       "B",
		ReplyEmail: "user@example.com",
		Status:     "new",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// sendgridPayload は SendGrid mail/send のリクエストボディのうち検証に必要な項目のみを表す。
type sendgridPayload struct {
	Personalizations []struct {
		To []struct {
			Email string `json:"email"`
		} `json:"to"`
	} `json:"personalizations"`
	Subject string `json:"subject"`
	Content []struct {
		Value string `json:"value"`
	} `json:"content"`
}

func TestSendInquiryReceipt(t *testing.T) {
	t.Run("受付確認メールの送信", func(t *testing.T) {
		t.Run("SendGrid が 202 を返すとき、宛先・件名・本文を載せた内容が送信されエラーにならない", func(t *testing.T) {
			received := make(chan sendgridPayload, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var payload sendgridPayload
				require.NoError(t, json.Unmarshal(raw, &payload))
				received <- payload
				w.WriteHeader(http.StatusAccepted)
			}))
			defer srv.Close()

			err := newTestSender(t, srv.URL).SendInquiryReceipt(context.Background(), newInquiry(dummyInquiryID), "抜粋本文")

			require.NoError(t, err)
			got := <-received
			require.Len(t, got.Personalizations, 1)
			require.Len(t, got.Personalizations[0].To, 1)
			assert.Equal(t, "user@example.com", got.Personalizations[0].To[0].Email)
			assert.Equal(t, fmt.Sprintf("[Overload Party サポート] 受付番号 #%d", dummyInquiryID), got.Subject)
			require.Len(t, got.Content, 1)
			assert.Contains(t, got.Content[0].Value, fmt.Sprintf("%d", dummyInquiryID))
			assert.Contains(t, got.Content[0].Value, "抜粋本文")
		})

		t.Run("SendGrid が 400 を返すとき、エラーになる", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer srv.Close()

			err := newTestSender(t, srv.URL).SendInquiryReceipt(context.Background(), newInquiry(dummyInquiryID), "抜粋本文")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "status=400")
		})
	})
}
