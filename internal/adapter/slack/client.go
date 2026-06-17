// Package slack は port.SlackNotifier の prod/staging 実装 (chat.postMessage API) を提供する。
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

var _ port.SlackNotifier = (*RealNotifier)(nil)

const chatPostMessageURL = "https://slack.com/api/chat.postMessage"

// RealNotifier は Slack bot token を使って運営チャンネルへ投稿する実装。
type RealNotifier struct {
	botToken  string
	channelID string
	client    *http.Client
}

// NewRealNotifier は RealNotifier を構築する。
func NewRealNotifier(botToken, channelID string) *RealNotifier {
	return &RealNotifier{
		botToken:  botToken,
		channelID: channelID,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyInquiryReceived は問い合わせ受付通知を運営チャンネルに投稿する。
func (n *RealNotifier) NotifyInquiryReceived(ctx context.Context, inquiry *domain.Inquiry, snippet string) error {
	body := map[string]any{
		"channel": n.channelID,
		"text":    fmt.Sprintf("新しい問い合わせ #%d", inquiry.InquiryID),
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]any{"type": "plain_text", "text": fmt.Sprintf("問い合わせ #%d", inquiry.InquiryID)}},
			{"type": "section", "fields": []map[string]any{
				{"type": "mrkdwn", "text": fmt.Sprintf("*件名*\n%s", inquiry.Title)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*返信先*\n%s", inquiry.ReplyEmail)},
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
	defer func() { _ = resp.Body.Close() }()

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

