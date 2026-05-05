package inquiry

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	"github.com/kenyamaneko/overload-party-support/internal/domain"
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

	inq, err := u.store.Create(ctx, title, body, replyEmail)
	if err != nil {
		return 0, fmt.Errorf("inquiry create: %w", err)
	}

	bodySnippet := snippet(inq.Body, u.snippetLength)

	if err := u.slack.NotifyInquiryReceived(ctx, inq, bodySnippet); err != nil {
		return 0, fmt.Errorf("slack notify: %w", err)
	}

	if err := u.email.SendInquiryReceipt(ctx, inq, bodySnippet); err != nil {
		return 0, fmt.Errorf("send receipt: %w", err)
	}

	return inq.InquiryID, nil
}

// List は status フィルタ付き一覧。空 slice はフィルタなし (全件)。
func (u *Usecase) List(ctx context.Context, statuses []string) ([]domain.Inquiry, error) {
	parsed, err := parseStatuses(statuses)
	if err != nil {
		return nil, err
	}
	items, err := u.store.List(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("inquiry list: %w", err)
	}
	return items, nil
}

// Get は ID 指定の詳細取得 (対応メモ含む)。
func (u *Usecase) Get(ctx context.Context, inquiryID int64) (*domain.Inquiry, error) {
	inq, err := u.store.Get(ctx, inquiryID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("inquiry get: %w", err)
	}
	return inq, nil
}

// UpdateStatus はステータスを更新する。
func (u *Usecase) UpdateStatus(ctx context.Context, inquiryID int64, newStatus string) (*domain.Inquiry, error) {
	target := newStatus
	if !domain.IsSupportedStatus(newStatus) {
		return nil, ErrInvalidStatusValue
	}
	if target == domain.StatusNew {
		return nil, ErrInvalidStatusTransition
	}

	current, err := u.store.Get(ctx, inquiryID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("inquiry get: %w", err)
	}
	if !isValidTransition(current.Status, target) {
		return nil, ErrInvalidStatusTransition
	}

	updated, err := u.store.UpdateStatus(ctx, inquiryID, target)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("inquiry update status: %w", err)
	}
	return updated, nil
}

// UpdateNote は対応メモを更新する。note が nil / 空文字は nil 扱い (メモ削除)。
func (u *Usecase) UpdateNote(ctx context.Context, inquiryID int64, note *string) (*domain.Inquiry, error) {
	var normalized *string
	if note != nil && *note != "" {
		normalized = note
	}
	updated, err := u.store.UpdateNote(ctx, inquiryID, normalized)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("inquiry update note: %w", err)
	}
	return updated, nil
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

// isValidTransition はステータス遷移表を実装する。
func isValidTransition(from, to string) bool {
	switch from {
	case domain.StatusNew:
		return to == domain.StatusInProgress || to == domain.StatusClosed
	case domain.StatusInProgress:
		return to == domain.StatusClosed
	case domain.StatusClosed:
		return false
	default:
		return false
	}
}

// parseStatuses は許容外の値が含まれればエラー。指定なし (空 slice / 空文字のみ) は全 Status 展開して返す。
func parseStatuses(ss []string) ([]string, error) {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s == "" {
			continue
		}
		if !domain.IsSupportedStatus(s) {
			return nil, ErrInvalidStatusValue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return domain.Statuses, nil
	}
	return out, nil
}

// snippet は文字列を rune 単位で先頭 n 文字に切り詰める。
func snippet(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return strings.TrimRight(string(runes[:n]), " ")
}
