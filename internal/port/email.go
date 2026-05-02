package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// EmailSender は問い合わせ者への受付確認メール送信 port。
type EmailSender interface {
	// SendInquiryReceipt は受付確認メールを replyEmail 宛に送る。
	SendInquiryReceipt(ctx context.Context, inquiry *domain.Inquiry, snippet string) error
}
