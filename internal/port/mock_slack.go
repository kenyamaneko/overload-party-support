package port

import (
	"context"

	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

// MockSlackNotifier は SlackNotifier のテスト用モック。
type MockSlackNotifier struct {
	NotifyInquiryReceivedFn func(ctx context.Context, inquiry *apisupport.Inquiry, snippet string) error
}

var _ SlackNotifier = (*MockSlackNotifier)(nil)

func (m *MockSlackNotifier) NotifyInquiryReceived(ctx context.Context, inquiry *apisupport.Inquiry, snippet string) error {
	if m.NotifyInquiryReceivedFn == nil {
		panic("MockSlackNotifier.NotifyInquiryReceived called without Fn")
	}
	return m.NotifyInquiryReceivedFn(ctx, inquiry, snippet)
}
