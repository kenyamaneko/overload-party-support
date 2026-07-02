package inquiry_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
)

// 仕様 (FEATURE_SPEC §7.1): Submit はバリデーション失敗時に副作用 (repo/slack/email) を一切呼ばない。
func TestSubmit_InputValidation(t *testing.T) {
	cases := []struct {
		name     string
		title    string
		body     string
		email    string
		wantErr  error
		wantCall bool
	}{
		{
			name:     "正常",
			title:    "T",
			body:     "B",
			email:    "user@example.com",
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "title 空",
			title:    "",
			body:     "B",
			email:    "user@example.com",
			wantErr:  inquiry.ErrInvalidInquiry,
			wantCall: false,
		},
		{
			name:     "body 空",
			title:    "T",
			body:     "",
			email:    "user@example.com",
			wantErr:  inquiry.ErrInvalidInquiry,
			wantCall: false,
		},
		{
			name:     "email 空",
			title:    "T",
			body:     "B",
			email:    "",
			wantErr:  inquiry.ErrInvalidInquiry,
			wantCall: false,
		},
		{
			name:     "title 上限超過 (101 文字)",
			title:    strings.Repeat("あ", 101),
			body:     "B",
			email:    "u@e.com",
			wantErr:  inquiry.ErrInvalidInquiry,
			wantCall: false,
		},
		{
			name:     "title 上限ちょうど (100 文字)",
			title:    strings.Repeat("あ", 100),
			body:     "B",
			email:    "u@e.com",
			wantErr:  nil,
			wantCall: true,
		},
		{
			name:     "body 上限超過 (4001 文字)",
			title:    "T",
			body:     strings.Repeat("あ", 4001),
			email:    "u@e.com",
			wantErr:  inquiry.ErrInvalidInquiry,
			wantCall: false,
		},
		{
			name:     "email 形式不正",
			title:    "T",
			body:     "B",
			email:    "not-an-email",
			wantErr:  inquiry.ErrInvalidEmail,
			wantCall: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var createCalled, slackCalled, emailCalled bool
			store := &port.MockInquiryStore{
				CreateFn: func(_ context.Context, _ string, _ string, _ string) (*domain.Inquiry, error) {
					createCalled = true
					return newInquiry(1), nil
				},
			}
			slack := &port.MockSlackNotifier{
				NotifyInquiryReceivedFn: func(_ context.Context, _ *domain.Inquiry, _ string) error {
					slackCalled = true
					return nil
				},
			}
			email := &port.MockEmailSender{
				SendInquiryReceiptFn: func(_ context.Context, _ *domain.Inquiry, _ string) error {
					emailCalled = true
					return nil
				},
			}

			_, err := inquiry.New(store, slack, email, 200).Submit(context.Background(), tc.title, tc.body, tc.email)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCall, createCalled, "repo create called")
			assert.Equal(t, tc.wantCall, slackCalled, "slack notify called")
			assert.Equal(t, tc.wantCall, emailCalled, "email send called")
		})
	}
}

// 仕様 (FEATURE_SPEC §7.1 / §7.3): Submit は DB → Slack → SendGrid の順で fail-fast。
func TestSubmit_FailFastSideEffects(t *testing.T) {
	dbErr := errors.New("db lost")
	slackErr := errors.New("slack 500")
	sendErr := errors.New("sendgrid 500")

	cases := []struct {
		name           string
		createResult   *domain.Inquiry
		createErr      error
		slackErr       error
		sendErr        error
		wantCreateCall bool
		wantSlackCall  bool
		wantEmailCall  bool
		wantErr        error
	}{
		{
			name:           "DB 失敗 → slack/email 呼ばない",
			createErr:      dbErr,
			wantCreateCall: true,
			wantSlackCall:  false,
			wantEmailCall:  false,
			wantErr:        dbErr,
		},
		{
			name:           "Slack 失敗 → email 呼ばない",
			createResult:   newInquiry(1),
			slackErr:       slackErr,
			wantCreateCall: true,
			wantSlackCall:  true,
			wantEmailCall:  false,
			wantErr:        slackErr,
		},
		{
			name:           "SendGrid 失敗 → エラーを透過",
			createResult:   newInquiry(1),
			sendErr:        sendErr,
			wantCreateCall: true,
			wantSlackCall:  true,
			wantEmailCall:  true,
			wantErr:        sendErr,
		},
		{
			name:           "全部成功",
			createResult:   newInquiry(1),
			wantCreateCall: true,
			wantSlackCall:  true,
			wantEmailCall:  true,
			wantErr:        nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var createCalled, slackCalled, emailCalled bool
			store := &port.MockInquiryStore{
				CreateFn: func(_ context.Context, _ string, _ string, _ string) (*domain.Inquiry, error) {
					createCalled = true
					return tc.createResult, tc.createErr
				},
			}
			slack := &port.MockSlackNotifier{
				NotifyInquiryReceivedFn: func(_ context.Context, _ *domain.Inquiry, _ string) error {
					slackCalled = true
					return tc.slackErr
				},
			}
			email := &port.MockEmailSender{
				SendInquiryReceiptFn: func(_ context.Context, _ *domain.Inquiry, _ string) error {
					emailCalled = true
					return tc.sendErr
				},
			}

			_, err := inquiry.New(store, slack, email, 200).Submit(context.Background(), "T", "B", "u@e.com")

			assert.Equal(t, tc.wantCreateCall, createCalled, "create called")
			assert.Equal(t, tc.wantSlackCall, slackCalled, "slack called")
			assert.Equal(t, tc.wantEmailCall, emailCalled, "email called")
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様 (FEATURE_SPEC §7.1 / §9.1): Slack 通知の snippet は body を先頭 N 文字に切り詰めて渡す。
func TestSubmit_SnippetLength(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		snippetLen  int
		wantSnippet string
	}{
		{
			name:        "body 短いのでそのまま",
			body:        "short",
			snippetLen:  200,
			wantSnippet: "short",
		},
		{
			name:        "ちょうど N 文字は切り詰めない",
			body:        strings.Repeat("あ", 200),
			snippetLen:  200,
			wantSnippet: strings.Repeat("あ", 200),
		},
		{
			name:        "N 文字超過は先頭のみ",
			body:        strings.Repeat("あ", 300),
			snippetLen:  50,
			wantSnippet: strings.Repeat("あ", 50),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gotSnippet string
			store := &port.MockInquiryStore{
				CreateFn: func(_ context.Context, _ string, body string, _ string) (*domain.Inquiry, error) {
					inq := newInquiry(1)
					inq.Body = body
					return inq, nil
				},
			}
			slack := &port.MockSlackNotifier{
				NotifyInquiryReceivedFn: func(_ context.Context, _ *domain.Inquiry, snippet string) error {
					gotSnippet = snippet
					return nil
				},
			}
			email := &port.MockEmailSender{
				SendInquiryReceiptFn: func(_ context.Context, _ *domain.Inquiry, _ string) error { return nil },
			}

			_, err := inquiry.New(store, slack, email, tc.snippetLen).Submit(context.Background(), "T", tc.body, "u@e.com")

			require.NoError(t, err)
			assert.Equal(t, tc.wantSnippet, gotSnippet)
		})
	}
}

func newInquiry(id int64) *domain.Inquiry {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	return &domain.Inquiry{
		InquiryID:  id,
		Title:      "T",
		Body:       "B",
		ReplyEmail: "u@e.com",
		Status:     "new",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
