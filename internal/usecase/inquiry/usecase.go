package inquiry

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/kenyamaneko/overload-party-support/internal/port"
)

const (
	titleMaxLen = 100
	bodyMaxLen  = 4000
)

// Usecase は問い合わせのユースケースを担う。
type Usecase struct {
	store         port.InquiryStore
	slack         port.SlackNotifier
	email         port.EmailSender
	snippetLength int
}

// New は Usecase を構築する。snippetLength は Slack 通知本文抜粋の文字数。
func New(store port.InquiryStore, slack port.SlackNotifier, email port.EmailSender, snippetLength int) *Usecase {
	return &Usecase{
		store:         store,
		slack:         slack,
		email:         email,
		snippetLength: snippetLength,
	}
}

// Submit は問い合わせ受付のオーケストレーションを行う。
func (u *Usecase) Submit(ctx context.Context, title, body, replyEmail string) (int64, error) {
	if err := validateSubmit(title, body, replyEmail); err != nil {
		return 0, err
	}

	inquiry, err := u.store.Create(ctx, title, body, replyEmail)
	if err != nil {
		return 0, fmt.Errorf("inquiry create: %w", err)
	}

	bodySnippet := truncate(inquiry.Body, u.snippetLength)

	if err := u.slack.NotifyInquiryReceived(ctx, inquiry, bodySnippet); err != nil {
		return 0, fmt.Errorf("slack notify: %w", err)
	}

	if err := u.email.SendInquiryReceipt(ctx, inquiry, bodySnippet); err != nil {
		return 0, fmt.Errorf("send receipt: %w", err)
	}

	return inquiry.InquiryID, nil
}

// validateSubmit は受付時のバリデーション。
func validateSubmit(title, body, replyEmail string) error {
	if title == "" || body == "" || replyEmail == "" {
		return ErrInvalidInquiry
	}
	if len([]rune(title)) > titleMaxLen {
		return ErrInvalidInquiry
	}
	if len([]rune(body)) > bodyMaxLen {
		return ErrInvalidInquiry
	}
	if _, err := mail.ParseAddress(replyEmail); err != nil {
		return ErrInvalidEmail
	}
	return nil
}

// truncate は文字列を rune 単位で先頭 n 文字に切り詰める。
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return strings.TrimRight(string(runes[:n]), " ")
}
