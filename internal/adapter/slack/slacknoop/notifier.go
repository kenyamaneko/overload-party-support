// Package slacknoop は local 環境用の no-op SlackNotifier を提供する。
// 外部送信は行わず slog にログするだけで、prod 用の chat.postMessage 実装 (adapter/slack)
// と同居させないことで「production binary に偽実装が混入する」レイヤ違反を避ける。
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
func (n *Notifier) NotifyInquiryReceived(_ context.Context, inq *domain.Inquiry, snippet string) error {
	slog.Info("slack noop: inquiry received",
		"inquiry_id", inq.InquiryID,
		"title", inq.Title,
		"reply_email", inq.ReplyEmail,
		"snippet", snippet,
	)
	return nil
}

var _ port.SlackNotifier = (*Notifier)(nil)
