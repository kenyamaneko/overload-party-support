package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// EmailSender は問い合わせ者への受付確認メール送信 port (FEATURE_SPEC §7.2)。
// 実装は adapter/sendgrid (prod/staging) または noop (local)。
// テンプレートは adapter 側で組み立てる (support バイナリ内 html/template)。
type EmailSender interface {
	// SendInquiryReceipt は受付確認メールを replyEmail 宛に送る。
	// snippet は本文抜粋 (service が config の snippetLength で計算済み)。
	// 抜粋ルールは業務仕様 (§7.2) なので service が SSoT として保持する。
	SendInquiryReceipt(ctx context.Context, inquiry *domain.Inquiry, snippet string) error
}
