package port

import (
	"context"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// EmailSender は問い合わせ者への受付確認メール送信 port (FEATURE_SPEC §7.2)。
// 実装は adapter/sendgrid (prod/staging) または mock (local)。
// テンプレートは adapter 側で組み立てる (support バイナリ内 html/template)。
type EmailSender interface {
	// SendInquiryReceipt は受付確認メールを replyEmail 宛に送る。
	SendInquiryReceipt(ctx context.Context, inquiry *apisupport.Inquiry) error
}
