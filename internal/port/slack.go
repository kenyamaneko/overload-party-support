package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// SlackNotifier は運営チャンネルへの投稿 port。
type SlackNotifier interface {
	// NotifyInquiryReceived は問い合わせ受付時の通知を送る。
	NotifyInquiryReceived(ctx context.Context, inquiry *domain.Inquiry, snippet string) error
}
