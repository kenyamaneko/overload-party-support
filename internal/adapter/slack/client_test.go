package slack

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
	dummyBotToken  = "xoxb-TST-0001"
	dummyChannelID = "C-TST-0001"

	dummyInquiryID int64 = 5
)

// rewriteHostTransport は Slack API 宛のリクエストを httptest.Server の host へ差し替える RoundTripper。
type rewriteHostTransport struct {
	target *url.URL
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// newTestNotifier は Slack API 応答先を httptest.Server に差し替えた RealNotifier を組み立てる。
func newTestNotifier(t *testing.T, serverURL string) *RealNotifier {
	t.Helper()
	target, err := url.Parse(serverURL)
	require.NoError(t, err)
	n := NewRealNotifier(dummyBotToken, dummyChannelID)
	n.client = &http.Client{Transport: &rewriteHostTransport{target: target}}
	return n
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

// receivedRequest は httptest.Server が受け取ったリクエストの検証に必要な項目。
type receivedRequest struct {
	auth string
	path string
	body map[string]any
	raw  string
}

func TestNotifyInquiryReceived(t *testing.T) {
	t.Run("問い合わせ受付通知の送信", func(t *testing.T) {
		t.Run("送ると、チャンネル・受付番号・本文抜粋を載せた投稿が送られエラーにならない", func(t *testing.T) {
			received := make(chan receivedRequest, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, json.Unmarshal(raw, &body))
				received <- receivedRequest{auth: r.Header.Get("Authorization"), path: r.URL.Path, body: body, raw: string(raw)}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()

			err := newTestNotifier(t, srv.URL).NotifyInquiryReceived(context.Background(), newInquiry(dummyInquiryID), "抜粋本文")

			require.NoError(t, err)
			got := <-received
			assert.Equal(t, "Bearer "+dummyBotToken, got.auth)
			assert.Equal(t, "/api/chat.postMessage", got.path)
			assert.Equal(t, dummyChannelID, got.body["channel"])
			assert.Contains(t, got.body["text"], fmt.Sprintf("%d", dummyInquiryID))
			assert.Contains(t, got.raw, "抜粋本文")
		})

		t.Run("HTTP 200でも応答がok:falseのとき、エラーになる", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
			}))
			defer srv.Close()

			err := newTestNotifier(t, srv.URL).NotifyInquiryReceived(context.Background(), newInquiry(dummyInquiryID), "抜粋本文")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid_auth")
		})

		t.Run("応答がJSONでないとき、エラーになる", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("not json"))
			}))
			defer srv.Close()

			err := newTestNotifier(t, srv.URL).NotifyInquiryReceived(context.Background(), newInquiry(dummyInquiryID), "抜粋本文")

			assert.Error(t, err)
		})
	})
}
