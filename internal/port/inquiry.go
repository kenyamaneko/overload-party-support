package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// InquiryStore は問い合わせの永続化 port。
// 受付 / 一覧 / 詳細 / ステータス更新 / メモ更新すべてこの port を経由する。
type InquiryStore interface {
	// Create は問い合わせを status = new で INSERT し、採番された ID と結果の行を返す (FEATURE_SPEC §7.1)。
	Create(ctx context.Context, title, body, replyEmail string) (*domain.Inquiry, error)

	// List は指定 statuses に一致する問い合わせを updated_at DESC で並べて返す (FEATURE_SPEC §8.3)。
	// 「全件」は呼び出し側が domain.Statuses 丸ごとを渡す契約で表現する (repo は分岐を持たない)。
	// 空 slice を渡した場合は 0 件返る。
	List(ctx context.Context, statuses []string) ([]domain.Inquiry, error)

	// Get は ID で単一の問い合わせを返す。存在しない場合は ErrNotFound。
	Get(ctx context.Context, inquiryID int64) (*domain.Inquiry, error)

	// UpdateStatus は status を更新する。許容しない状態遷移は呼び出し側 service で事前検証する前提。
	UpdateStatus(ctx context.Context, inquiryID int64, newStatus string) (*domain.Inquiry, error)

	// UpdateNote は internal_note を更新する (nil / 空で削除扱い)。
	UpdateNote(ctx context.Context, inquiryID int64, note *string) (*domain.Inquiry, error)
}
