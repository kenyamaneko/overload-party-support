// Package sendgrid は port.EmailSender の実装を提供する。
// prod/staging では SendGrid v3 API を叩く RealSender、local では MockSender を使う。
// テンプレートは support バイナリ内の html/template で組み立て、SendGrid 管理画面には置かない
// (ARCHITECTURE.md「Slack / SendGrid クライアントの注入境界」)。
package sendgrid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/port"
	apisupport "github.com/kenyamaneko/overload-party-support/packages/api-support"
)

var _ port.EmailSender = (*RealSender)(nil)

const mailSendURL = "https://api.sendgrid.com/v3/mail/send"

// receiptTemplate は受付確認メールの日本語固定テンプレート (FEATURE_SPEC §7.2)。
const receiptTemplate = `{{.ReplyEmail}} 様

お問い合わせを受け付けました。担当者から改めてご連絡いたします。

──────────
受付ID : {{.InquiryID}}
受付日時: {{.CreatedAt.Format "2006-01-02 15:04:05 MST"}}
件名    : {{.Title}}
──────────

{{.BodySnippet}}

──────────
本メールは自動送信です。このメールに直接返信されても返答できません。
`

const subjectTemplate = "[Overload Party サポート] 受付番号 #{{.InquiryID}}"

// RealSender は SendGrid API key を使って受付確認メールを送る実装。
type RealSender struct {
	apiKey      string
	fromAddress string
	fromName    string
	client      *http.Client
	bodyTmpl    *template.Template
	subjectTmpl *template.Template
}

// NewRealSender は RealSender を構築する。
func NewRealSender(apiKey, fromAddress, fromName string) (*RealSender, error) {
	body, err := template.New("receipt").Parse(receiptTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse receipt template: %w", err)
	}
	subject, err := template.New("subject").Parse(subjectTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse subject template: %w", err)
	}
	return &RealSender{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		fromName:    fromName,
		client:      &http.Client{Timeout: 10 * time.Second},
		bodyTmpl:    body,
		subjectTmpl: subject,
	}, nil
}

// SendInquiryReceipt は受付確認メールを SendGrid 経由で送信する (FEATURE_SPEC §7.2)。
func (s *RealSender) SendInquiryReceipt(ctx context.Context, inq *apisupport.Inquiry) error {
	bodyText, err := s.renderBody(inq)
	if err != nil {
		return err
	}
	subject, err := s.renderSubject(inq)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": inq.ReplyEmail}}},
		},
		"from":    map[string]string{"email": s.fromAddress, "name": s.fromName},
		"subject": subject,
		"content": []map[string]string{
			{"type": "text/plain", "value": bodyText},
		},
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sendgrid payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mailSendURL, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build sendgrid request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send sendgrid request: %w", err)
	}
	defer resp.Body.Close()

	// SendGrid は成功時 202 Accepted、失敗時は 4xx/5xx + JSON エラーボディを返す。
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid api error: status=%d", resp.StatusCode)
	}
	return nil
}

// renderBody は本文テンプレートを評価する。
func (s *RealSender) renderBody(inq *apisupport.Inquiry) (string, error) {
	data := templateData(inq)
	var buf bytes.Buffer
	if err := s.bodyTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render receipt body: %w", err)
	}
	return buf.String(), nil
}

// renderSubject は件名テンプレートを評価する。
func (s *RealSender) renderSubject(inq *apisupport.Inquiry) (string, error) {
	data := templateData(inq)
	var buf bytes.Buffer
	if err := s.subjectTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render receipt subject: %w", err)
	}
	return buf.String(), nil
}

// templateData はテンプレート評価用のビューモデル (snippet を本文から導出)。
func templateData(inq *apisupport.Inquiry) map[string]any {
	const snippetLen = 400
	runes := []rune(inq.Body)
	snippet := inq.Body
	if len(runes) > snippetLen {
		snippet = string(runes[:snippetLen]) + "…"
	}
	return map[string]any{
		"InquiryID":   inq.InquiryID,
		"Title":       inq.Title,
		"ReplyEmail":  inq.ReplyEmail,
		"CreatedAt":   inq.CreatedAt,
		"BodySnippet": snippet,
	}
}

// MockSender は local 環境用のダミー実装。外部送信は行わない。
type MockSender struct{}

// NewMockSender は MockSender を生成する。
func NewMockSender() *MockSender { return &MockSender{} }

// SendInquiryReceipt はログ出力のみ。
func (s *MockSender) SendInquiryReceipt(ctx context.Context, inq *apisupport.Inquiry) error {
	slog.Info("sendgrid mock: receipt email",
		"inquiry_id", inq.InquiryID,
		"to", inq.ReplyEmail,
		"title", inq.Title,
	)
	return nil
}

var _ port.EmailSender = (*MockSender)(nil)
