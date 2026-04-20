package port

import (
	"context"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// MockEmailSender は EmailSender のテスト用モック。
type MockEmailSender struct {
	SendInquiryReceiptFn func(ctx context.Context, inquiry *apisupport.Inquiry) error
}

var _ EmailSender = (*MockEmailSender)(nil)

func (m *MockEmailSender) SendInquiryReceipt(ctx context.Context, inquiry *apisupport.Inquiry) error {
	if m.SendInquiryReceiptFn == nil {
		panic("MockEmailSender.SendInquiryReceipt called without Fn")
	}
	return m.SendInquiryReceiptFn(ctx, inquiry)
}
