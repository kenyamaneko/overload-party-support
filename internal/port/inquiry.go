package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// InquiryStore は問い合わせの永続化 port。
type InquiryStore interface {
	// Create は問い合わせを status = new で INSERT し、採番された ID と結果の行を返す。
	Create(ctx context.Context, title, body, replyEmail string) (*domain.Inquiry, error)

	// List は指定 statuses に一致する問い合わせを updated_at DESC で並べて返す。
	List(ctx context.Context, statuses []string) ([]domain.Inquiry, error)

	// Get は ID で単一の問い合わせを返す。存在しない場合は ErrNotFound。
	Get(ctx context.Context, inquiryID int64) (*domain.Inquiry, error)

	// UpdateStatus は status を更新する。
	UpdateStatus(ctx context.Context, inquiryID int64, newStatus string) (*domain.Inquiry, error)

	// UpdateNote は internal_note を更新する (nil / 空で削除扱い)。
	UpdateNote(ctx context.Context, inquiryID int64, note *string) (*domain.Inquiry, error)
}
