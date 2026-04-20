package port

import (
	"context"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// MockInquiryStore は InquiryStore のテスト用モック。
type MockInquiryStore struct {
	CreateFn       func(ctx context.Context, title, body, replyEmail string) (*apisupport.Inquiry, error)
	ListFn         func(ctx context.Context, statuses []apisupport.Status) ([]apisupport.Inquiry, error)
	GetFn          func(ctx context.Context, inquiryID int64) (*apisupport.Inquiry, error)
	UpdateStatusFn func(ctx context.Context, inquiryID int64, newStatus apisupport.Status) (*apisupport.Inquiry, error)
	UpdateNoteFn   func(ctx context.Context, inquiryID int64, note *string) (*apisupport.Inquiry, error)
}

var _ InquiryStore = (*MockInquiryStore)(nil)

func (m *MockInquiryStore) Create(ctx context.Context, title, body, replyEmail string) (*apisupport.Inquiry, error) {
	if m.CreateFn == nil {
		panic("MockInquiryStore.Create called without Fn")
	}
	return m.CreateFn(ctx, title, body, replyEmail)
}

func (m *MockInquiryStore) List(ctx context.Context, statuses []apisupport.Status) ([]apisupport.Inquiry, error) {
	if m.ListFn == nil {
		panic("MockInquiryStore.List called without Fn")
	}
	return m.ListFn(ctx, statuses)
}

func (m *MockInquiryStore) Get(ctx context.Context, inquiryID int64) (*apisupport.Inquiry, error) {
	if m.GetFn == nil {
		panic("MockInquiryStore.Get called without Fn")
	}
	return m.GetFn(ctx, inquiryID)
}

func (m *MockInquiryStore) UpdateStatus(ctx context.Context, inquiryID int64, newStatus apisupport.Status) (*apisupport.Inquiry, error) {
	if m.UpdateStatusFn == nil {
		panic("MockInquiryStore.UpdateStatus called without Fn")
	}
	return m.UpdateStatusFn(ctx, inquiryID, newStatus)
}

func (m *MockInquiryStore) UpdateNote(ctx context.Context, inquiryID int64, note *string) (*apisupport.Inquiry, error) {
	if m.UpdateNoteFn == nil {
		panic("MockInquiryStore.UpdateNote called without Fn")
	}
	return m.UpdateNoteFn(ctx, inquiryID, note)
}
