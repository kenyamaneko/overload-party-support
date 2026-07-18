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

func TestSubmit(t *testing.T) {
	t.Run("問い合わせの送信", func(t *testing.T) {
		t.Run("入力バリデーション", func(t *testing.T) {
			validCases := []struct {
				name  string
				title string
				body  string
				email string
			}{
				{
					name:  "全項目が正常のとき、作成され repo・slack・email が呼ばれる",
					title: "T",
					body:  "B",
					email: "user@example.com",
				},
				{
					name:  "title が上限ちょうど (100 文字) のとき、作成され repo・slack・email が呼ばれる",
					title: strings.Repeat("あ", 100),
					body:  "B",
					email: "u@e.com",
				},
			}
			for _, tc := range validCases {
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

					require.NoError(t, err)
					assert.True(t, createCalled, "repo create called")
					assert.True(t, slackCalled, "slack notify called")
					assert.True(t, emailCalled, "email send called")
				})
			}

			invalidCases := []struct {
				name    string
				title   string
				body    string
				email   string
				wantErr error
			}{
				{
					name:    "title が空のとき、ErrInvalidInquiry になり副作用は呼ばれない",
					title:   "",
					body:    "B",
					email:   "user@example.com",
					wantErr: inquiry.ErrInvalidInquiry,
				},
				{
					name:    "body が空のとき、ErrInvalidInquiry になり副作用は呼ばれない",
					title:   "T",
					body:    "",
					email:   "user@example.com",
					wantErr: inquiry.ErrInvalidInquiry,
				},
				{
					name:    "email が空のとき、ErrInvalidInquiry になり副作用は呼ばれない",
					title:   "T",
					body:    "B",
					email:   "",
					wantErr: inquiry.ErrInvalidInquiry,
				},
				{
					name:    "title が上限超過 (101 文字) のとき、ErrInvalidInquiry になり副作用は呼ばれない",
					title:   strings.Repeat("あ", 101),
					body:    "B",
					email:   "u@e.com",
					wantErr: inquiry.ErrInvalidInquiry,
				},
				{
					name:    "body が上限超過 (4001 文字) のとき、ErrInvalidInquiry になり副作用は呼ばれない",
					title:   "T",
					body:    strings.Repeat("あ", 4001),
					email:   "u@e.com",
					wantErr: inquiry.ErrInvalidInquiry,
				},
				{
					name:    "email が形式不正のとき、ErrInvalidEmail になり副作用は呼ばれない",
					title:   "T",
					body:    "B",
					email:   "not-an-email",
					wantErr: inquiry.ErrInvalidEmail,
				},
			}
			for _, tc := range invalidCases {
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
					assert.False(t, createCalled, "repo create called")
					assert.False(t, slackCalled, "slack notify called")
					assert.False(t, emailCalled, "email send called")
				})
			}
		})

		t.Run("副作用の fail-fast", func(t *testing.T) {
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
					name:           "repo create が失敗するとき、slack・email は呼ばれずそのエラーが返る",
					createErr:      dbErr,
					wantCreateCall: true,
					wantSlackCall:  false,
					wantEmailCall:  false,
					wantErr:        dbErr,
				},
				{
					name:           "slack 通知が失敗するとき、email は呼ばれずそのエラーが返る",
					createResult:   newInquiry(1),
					slackErr:       slackErr,
					wantCreateCall: true,
					wantSlackCall:  true,
					wantEmailCall:  false,
					wantErr:        slackErr,
				},
				{
					name:           "email 送信が失敗するとき、そのエラーが透過される",
					createResult:   newInquiry(1),
					sendErr:        sendErr,
					wantCreateCall: true,
					wantSlackCall:  true,
					wantEmailCall:  true,
					wantErr:        sendErr,
				},
				{
					name:           "全ステップ成功のとき、repo・slack・email が呼ばれエラーにならない",
					createResult:   newInquiry(1),
					wantCreateCall: true,
					wantSlackCall:  true,
					wantEmailCall:  true,
					wantErr:        nil,
				},
			}

			for _, tc := range cases {
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
		})

		t.Run("Slack snippet の切り詰め", func(t *testing.T) {
			cases := []struct {
				name        string
				body        string
				snippetLen  int
				wantSnippet string
			}{
				{
					name:        "body が snippet 長より短いとき、そのまま渡される",
					body:        "short",
					snippetLen:  200,
					wantSnippet: "short",
				},
				{
					name:        "body が snippet 長ちょうど (200 文字) のとき、切り詰めない",
					body:        strings.Repeat("あ", 200),
					snippetLen:  200,
					wantSnippet: strings.Repeat("あ", 200),
				},
				{
					name:        "body が snippet 長超過のとき、先頭 (50 文字) のみ渡される",
					body:        strings.Repeat("あ", 300),
					snippetLen:  50,
					wantSnippet: strings.Repeat("あ", 50),
				},
				{
					name:        "切り詰め位置が空白に当たるとき、末尾の空白を除いた抜粋が Slack 通知に渡される",
					body:        "abcd efgh",
					snippetLen:  5,
					wantSnippet: "abcd",
				},
			}

			for _, tc := range cases {
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
		})
	})
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
