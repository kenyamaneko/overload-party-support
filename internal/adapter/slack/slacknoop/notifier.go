// Package slacknoop は local 環境用の no-op SlackNotifier を提供する。
package slacknoop

import (
	"context"
	"log/slog"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

// Notifier は SlackNotifier の no-op 実装。
type Notifier struct{}

// New は Notifier を生成する。
func New() *Notifier { return &Notifier{} }

// NotifyInquiryReceived はログ出力のみ。
func (n *Notifier) NotifyInquiryReceived(_ context.Context, inquiry *domain.Inquiry, snippet string) error {
	slog.Info("slack noop: inquiry received",
		"inquiry_id", inquiry.InquiryID,
		"title", inquiry.Title,
		"reply_email", inquiry.ReplyEmail,
		"snippet", snippet,
	)
	return nil
}

var _ port.SlackNotifier = (*Notifier)(nil)
