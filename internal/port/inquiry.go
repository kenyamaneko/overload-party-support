package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// InquiryStore は問い合わせの永続化 port。
type InquiryStore interface {
	// Create は問い合わせを status = new で INSERT し、採番された ID と結果の行を返す。
	Create(ctx context.Context, title, body, replyEmail string) (*domain.Inquiry, error)
}
