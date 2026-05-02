package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// MockEmailSender は EmailSender のテスト用モック。
type MockEmailSender struct {
	SendInquiryReceiptFn func(ctx context.Context, inquiry *domain.Inquiry, snippet string) error
}

var _ EmailSender = (*MockEmailSender)(nil)

func (m *MockEmailSender) SendInquiryReceipt(ctx context.Context, inquiry *domain.Inquiry, snippet string) error {
	if m.SendInquiryReceiptFn == nil {
		panic("MockEmailSender.SendInquiryReceipt called without Fn")
	}
	return m.SendInquiryReceiptFn(ctx, inquiry, snippet)
}
