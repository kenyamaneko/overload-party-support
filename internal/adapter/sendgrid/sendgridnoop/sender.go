// Package sendgridnoop は local 環境用の no-op EmailSender を提供する。
package sendgridnoop

import (
	"context"
	"log/slog"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

// Sender は EmailSender の no-op 実装。
type Sender struct{}

// New は Sender を生成する。
func New() *Sender { return &Sender{} }

// SendInquiryReceipt はログ出力のみ。
func (s *Sender) SendInquiryReceipt(_ context.Context, inq *domain.Inquiry, snippet string) error {
	slog.Info("sendgrid noop: receipt email",
		"inquiry_id", inq.InquiryID,
		"to", inq.ReplyEmail,
		"title", inq.Title,
		"snippet", snippet,
	)
	return nil
}

var _ port.EmailSender = (*Sender)(nil)
