package port

import (
	"context"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
)

// MockInquiryStore は InquiryStore のテスト用モック。
type MockInquiryStore struct {
	CreateFn func(ctx context.Context, title, body, replyEmail string) (*domain.Inquiry, error)
}

var _ InquiryStore = (*MockInquiryStore)(nil)

func (m *MockInquiryStore) Create(ctx context.Context, title, body, replyEmail string) (*domain.Inquiry, error) {
	if m.CreateFn == nil {
		panic("MockInquiryStore.Create called without Fn")
	}
	return m.CreateFn(ctx, title, body, replyEmail)
}
