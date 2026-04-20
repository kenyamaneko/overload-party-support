// Package slack は port.SlackNotifier の実装を提供する。
// prod/staging では chat.postMessage API を叩く RealNotifier、local では MockNotifier を使う。
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

var _ port.SlackNotifier = (*RealNotifier)(nil)

const chatPostMessageURL = "https://slack.com/api/chat.postMessage"

// RealNotifier は Slack bot token を使って運営チャンネルへ投稿する実装。
type RealNotifier struct {
	botToken  string
	channelID string
	client    *http.Client
}

// NewRealNotifier は RealNotifier を構築する。token / channelID は空であってはならない (config で検証済み)。
func NewRealNotifier(botToken, channelID string) *RealNotifier {
	return &RealNotifier{
		botToken:  botToken,
		channelID: channelID,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyInquiryReceived は問い合わせ受付通知を運営チャンネルに投稿する (FEATURE_SPEC §9.1)。
// Block Kit 構造と interactive button の action_id 命名は slack-commands 側の責務なので、
// ここではプレーンな Block メッセージだけを送る (button は slack-commands で後付けされる想定)。
func (n *RealNotifier) NotifyInquiryReceived(ctx context.Context, inq *apisupport.Inquiry, snippet string) error {
	body := map[string]any{
		"channel": n.channelID,
		"text":    fmt.Sprintf("新しい問い合わせ #%d", inq.InquiryID),
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]any{"type": "plain_text", "text": fmt.Sprintf("問い合わせ #%d", inq.InquiryID)}},
			{"type": "section", "fields": []map[string]any{
				{"type": "mrkdwn", "text": fmt.Sprintf("*件名*\n%s", inq.Title)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*返信先*\n%s", inq.ReplyEmail)},
			}},
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*本文抜粋*\n%s", snippet)}},
		},
	}
	return n.postJSON(ctx, chatPostMessageURL, body)
}

// postJSON は Slack Web API に JSON を POST し、ok=false なら error を返す。
func (n *RealNotifier) postJSON(ctx context.Context, url string, payload any) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+n.botToken)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode slack response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack api error: %s", result.Error)
	}
	return nil
}

// MockNotifier は local 環境用のダミー実装。ログ出力のみで外部送信は行わない。
type MockNotifier struct{}

// NewMockNotifier は MockNotifier を生成する。
func NewMockNotifier() *MockNotifier { return &MockNotifier{} }

// NotifyInquiryReceived はログに出すだけで成功を返す。
func (n *MockNotifier) NotifyInquiryReceived(ctx context.Context, inq *apisupport.Inquiry, snippet string) error {
	slog.Info("slack mock: inquiry received",
		"inquiry_id", inq.InquiryID,
		"title", inq.Title,
		"reply_email", inq.ReplyEmail,
		"snippet", snippet,
	)
	return nil
}

var _ port.SlackNotifier = (*MockNotifier)(nil)
