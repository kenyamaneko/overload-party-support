package inquiry_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/usecase/inquiry"
	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// 仕様 (FEATURE_SPEC §7.1): Submit は必須・長さ・email 形式をバリデートし、失敗時は repo/slack/email を呼ばない。
func TestSubmit_仕様_入力バリデーション(t *testing.T) {
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
					return newInquiry(1, domain.StatusNew), nil
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
// Slack 失敗時は SendGrid を呼ばない、DB 失敗時は Slack を呼ばない。
func TestSubmit_仕様_副作用はfailfast(t *testing.T) {
	dbErr := errors.New("db lost")
	slackErr := errors.New("slack 500")
	sendErr := errors.New("sendgrid 500")

	cases := []struct {
		name            string
		createErr       error
		slackErr        error
		sendErr         error
		wantCreateCall  bool
		wantSlackCall   bool
		wantEmailCall   bool
		wantErrContains string
	}{
		{
			name:            "DB 失敗 → slack/email 呼ばない",
			createErr:       dbErr,
			wantCreateCall:  true,
			wantSlackCall:   false,
			wantEmailCall:   false,
			wantErrContains: "db lost",
		},
		{
			name:            "Slack 失敗 → email 呼ばない",
			slackErr:        slackErr,
			wantCreateCall:  true,
			wantSlackCall:   true,
			wantEmailCall:   false,
			wantErrContains: "slack 500",
		},
		{
			name:            "SendGrid 失敗 → エラーを透過",
			sendErr:         sendErr,
			wantCreateCall:  true,
			wantSlackCall:   true,
			wantEmailCall:   true,
			wantErrContains: "sendgrid 500",
		},
		{
			name:            "全部成功",
			wantCreateCall:  true,
			wantSlackCall:   true,
			wantEmailCall:   true,
			wantErrContains: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var createCalled, slackCalled, emailCalled bool
			store := &port.MockInquiryStore{
				CreateFn: func(_ context.Context, _ string, _ string, _ string) (*domain.Inquiry, error) {
					createCalled = true
					if tc.createErr != nil {
						return nil, tc.createErr
					}
					return newInquiry(1, domain.StatusNew), nil
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
			switch tc.wantErrContains {
			case "":
				assert.NoError(t, err)
			default:
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
			}
		})
	}
}

// 仕様 (FEATURE_SPEC §7.1 / §9.1): Slack 通知の snippet は body を先頭 N 文字に切り詰めて渡す。
func TestSubmit_仕様_snippetは指定文字数(t *testing.T) {
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
					inq := newInquiry(1, domain.StatusNew)
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

// 仕様 (FEATURE_SPEC §8.1): ステータス遷移表を厳密に守る。
// - `new` への遷移は常に不可 (server 初期値のみ)
// - `closed` からの逆遷移は不可
// - `new` → `in_progress` / `closed` OK
// - `in_progress` → `closed` OK, `new` への戻しは不可
func TestUpdateStatus_仕様_遷移表(t *testing.T) {
	cases := []struct {
		name      string
		current   string
		target    string
		wantErr   error
		wantWrite bool
	}{
		{
			name:      "new → in_progress",
			current:   domain.StatusNew,
			target:    domain.StatusInProgress,
			wantErr:   nil,
			wantWrite: true,
		},
		{
			name:      "new → closed",
			current:   domain.StatusNew,
			target:    domain.StatusClosed,
			wantErr:   nil,
			wantWrite: true,
		},
		{
			name:      "in_progress → closed",
			current:   domain.StatusInProgress,
			target:    domain.StatusClosed,
			wantErr:   nil,
			wantWrite: true,
		},
		{
			name:      "new → new 不可",
			current:   domain.StatusNew,
			target:    domain.StatusNew,
			wantErr:   inquiry.ErrInvalidStatusTransition,
			wantWrite: false,
		},
		{
			name:      "in_progress → new 不可",
			current:   domain.StatusInProgress,
			target:    domain.StatusNew,
			wantErr:   inquiry.ErrInvalidStatusTransition,
			wantWrite: false,
		},
		{
			name:      "closed → in_progress 不可",
			current:   domain.StatusClosed,
			target:    domain.StatusInProgress,
			wantErr:   inquiry.ErrInvalidStatusTransition,
			wantWrite: false,
		},
		{
			name:      "closed → closed 不可",
			current:   domain.StatusClosed,
			target:    domain.StatusClosed,
			wantErr:   inquiry.ErrInvalidStatusTransition,
			wantWrite: false,
		},
		{
			name:      "closed → new 不可",
			current:   domain.StatusClosed,
			target:    domain.StatusNew,
			wantErr:   inquiry.ErrInvalidStatusTransition,
			wantWrite: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var wrote bool
			store := &port.MockInquiryStore{
				GetFn: func(_ context.Context, _ int64) (*domain.Inquiry, error) {
					return newInquiry(1, tc.current), nil
				},
				UpdateStatusFn: func(_ context.Context, _ int64, _ string) (*domain.Inquiry, error) {
					wrote = true
					return newInquiry(1, tc.target), nil
				},
			}
			slack := &port.MockSlackNotifier{}
			email := &port.MockEmailSender{}

			_, err := inquiry.New(store, slack, email, 200).UpdateStatus(context.Background(), 1, string(tc.target))

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantWrite, wrote)
		})
	}
}

// 仕様: UpdateStatus は未知の status 値を ErrInvalidStatusValue、非存在を ErrNotFound にマップする。
func TestUpdateStatus_仕様_エラーマッピング(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		getErr  error
		wantErr error
	}{
		{
			name:    "未知値は ErrInvalidStatusValue",
			target:  "unknown",
			wantErr: inquiry.ErrInvalidStatusValue,
		},
		{
			name:    "get で not found → ErrNotFound",
			target:  "in_progress",
			getErr:  port.ErrNotFound,
			wantErr: inquiry.ErrNotFound,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &port.MockInquiryStore{
				GetFn: func(_ context.Context, _ int64) (*domain.Inquiry, error) {
					if tc.getErr != nil {
						return nil, tc.getErr
					}
					return newInquiry(1, domain.StatusNew), nil
				},
				UpdateStatusFn: func(_ context.Context, _ int64, _ string) (*domain.Inquiry, error) {
					return newInquiry(1, domain.StatusInProgress), nil
				},
			}
			slack := &port.MockSlackNotifier{}
			email := &port.MockEmailSender{}

			_, err := inquiry.New(store, slack, email, 200).UpdateStatus(context.Background(), 1, tc.target)

			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 仕様: UpdateNote は nil / 空文字を同一視してメモ削除扱いにする。
func TestUpdateNote_仕様_空文字は削除扱い(t *testing.T) {
	empty := ""
	nonEmpty := "対応中: 追加情報待ち"

	cases := []struct {
		name     string
		input    *string
		wantPass *string // repo に渡る値 (nil/非nil)
	}{
		{
			name:     "nil は nil として渡す",
			input:    nil,
			wantPass: nil,
		},
		{
			name:     "空文字は nil に正規化",
			input:    &empty,
			wantPass: nil,
		},
		{
			name:     "非空はそのまま",
			input:    &nonEmpty,
			wantPass: &nonEmpty,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gotNote *string
			store := &port.MockInquiryStore{
				UpdateNoteFn: func(_ context.Context, _ int64, note *string) (*domain.Inquiry, error) {
					gotNote = note
					return newInquiry(1, domain.StatusNew), nil
				},
			}
			slack := &port.MockSlackNotifier{}
			email := &port.MockEmailSender{}

			_, err := inquiry.New(store, slack, email, 200).UpdateNote(context.Background(), 1, tc.input)

			require.NoError(t, err)
			switch {
			case tc.wantPass == nil:
				assert.Nil(t, gotNote)
			default:
				require.NotNil(t, gotNote)
				assert.Equal(t, *tc.wantPass, *gotNote)
			}
		})
	}
}

// 仕様 (FEATURE_SPEC §8.3): List は status クエリをパースし、未知値を ErrInvalidStatusValue として早期 fail する。
// 指定なし (nil / 空文字のみ) は domain.Statuses (全 Status) に展開して repo に渡す。
func TestList_仕様_statusパース(t *testing.T) {
	allLen := len(domain.Statuses)

	cases := []struct {
		name        string
		statuses    []string
		wantErr     error
		wantCalled  bool
		wantPassLen int
	}{
		{
			name:        "nil slice は全 Status に展開",
			statuses:    nil,
			wantErr:     nil,
			wantCalled:  true,
			wantPassLen: allLen,
		},
		{
			name:        "空文字のみも全 Status に展開",
			statuses:    []string{""},
			wantErr:     nil,
			wantCalled:  true,
			wantPassLen: allLen,
		},
		{
			name:        "new 単体",
			statuses:    []string{"new"},
			wantErr:     nil,
			wantCalled:  true,
			wantPassLen: 1,
		},
		{
			name:        "new + in_progress",
			statuses:    []string{"new", "in_progress"},
			wantErr:     nil,
			wantCalled:  true,
			wantPassLen: 2,
		},
		{
			name:        "空文字混在は残りのみ渡す",
			statuses:    []string{"", "new", ""},
			wantErr:     nil,
			wantCalled:  true,
			wantPassLen: 1,
		},
		{
			name:        "未知値は ErrInvalidStatusValue",
			statuses:    []string{"new", "invalid"},
			wantErr:     inquiry.ErrInvalidStatusValue,
			wantCalled:  false,
			wantPassLen: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			var gotStatuses []string
			store := &port.MockInquiryStore{
				ListFn: func(_ context.Context, s []string) ([]domain.Inquiry, error) {
					called = true
					gotStatuses = s
					return nil, nil
				},
			}
			_, err := inquiry.New(store, nil, nil, 200).List(context.Background(), tc.statuses)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantCalled, called)
			assert.Len(t, gotStatuses, tc.wantPassLen)
		})
	}
}

// 仕様: Get は port.ErrNotFound を service の ErrNotFound にマップ、それ以外は透過。
func TestGet_仕様_NotFoundマップ(t *testing.T) {
	dbErr := errors.New("db lost")

	cases := []struct {
		name    string
		getErr  error
		wantErr error
	}{
		{
			name:    "正常",
			getErr:  nil,
			wantErr: nil,
		},
		{
			name:    "port.ErrNotFound → ErrNotFound",
			getErr:  port.ErrNotFound,
			wantErr: inquiry.ErrNotFound,
		},
		{
			name:    "その他は透過",
			getErr:  dbErr,
			wantErr: dbErr,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &port.MockInquiryStore{
				GetFn: func(_ context.Context, _ int64) (*domain.Inquiry, error) {
					if tc.getErr != nil {
						return nil, tc.getErr
					}
					return newInquiry(1, domain.StatusNew), nil
				},
			}
			_, err := inquiry.New(store, nil, nil, 200).Get(context.Background(), 1)

			switch tc.wantErr {
			case nil:
				assert.NoError(t, err)
			default:
				assert.ErrorContains(t, err, errMessage(tc.wantErr))
			}
		})
	}
}

// 仕様: UpdateStatus / UpdateNote は更新時の port.ErrNotFound も ErrNotFound にマップする。
// (GetFn 段階の NotFound は別ケースで検証済み。ここは Get 成功後に UpdateStatusFn / UpdateNoteFn が NotFound を返すレース)
func TestUpdateStatusNote_仕様_書き込み時のNotFound(t *testing.T) {
	cases := []struct {
		name        string
		run         func(uc *inquiry.Usecase) error
		injectStore func() *port.MockInquiryStore
	}{
		{
			name: "UpdateStatus: UpdateStatusFn が NotFound",
			run: func(uc *inquiry.Usecase) error {
				_, err := uc.UpdateStatus(context.Background(), 1, "in_progress")
				return err
			},
			injectStore: func() *port.MockInquiryStore {
				return &port.MockInquiryStore{
					GetFn: func(_ context.Context, _ int64) (*domain.Inquiry, error) {
						return newInquiry(1, domain.StatusNew), nil
					},
					UpdateStatusFn: func(_ context.Context, _ int64, _ string) (*domain.Inquiry, error) {
						return nil, port.ErrNotFound
					},
				}
			},
		},
		{
			name: "UpdateNote: UpdateNoteFn が NotFound",
			run: func(uc *inquiry.Usecase) error {
				_, err := uc.UpdateNote(context.Background(), 1, nil)
				return err
			},
			injectStore: func() *port.MockInquiryStore {
				return &port.MockInquiryStore{
					UpdateNoteFn: func(_ context.Context, _ int64, _ *string) (*domain.Inquiry, error) {
						return nil, port.ErrNotFound
					},
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := tc.injectStore()
			uc := inquiry.New(store, nil, nil, 200)
			err := tc.run(uc)
			assert.ErrorIs(t, err, inquiry.ErrNotFound)
		})
	}
}

// errMessage はエラーメッセージを取り出す (センチネル・一般 error どちらも扱うための helper)。
func errMessage(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func newInquiry(id int64, status string) *domain.Inquiry {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	return &domain.Inquiry{
		InquiryID:  id,
		Title:      "T",
		Body:       "B",
		ReplyEmail: "u@e.com",
		Status:     status,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
