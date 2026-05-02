package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// SlackNotifier は運営チャンネルへの投稿 port (FEATURE_SPEC §9.1)。
// 実装は adapter/slack (prod/staging) または adapter/slack の mock 実装 (local)。
// service 層は Block Kit / interactive payload を知らない。
type SlackNotifier interface {
	// NotifyInquiryReceived は問い合わせ受付時の通知を送る。
	// snippet は `body` の先頭 N 文字 (N は config.InquiryBodySnippetLength)。
	NotifyInquiryReceived(ctx context.Context, inquiry *domain.Inquiry, snippet string) error
}
