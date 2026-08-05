// Package sendgrid は port.EmailSender の実装を提供する。
package sendgrid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/kenyamaneko/overload-party-support/internal/domain"
	"github.com/kenyamaneko/overload-party-support/internal/port"
)

var _ port.EmailSender = (*RealSender)(nil)

const mailSendURL = "https://api.sendgrid.com/v3/mail/send"

// receiptTemplate は受付確認メールの日本語固定テンプレート。
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

// SendInquiryReceipt は受付確認メールを SendGrid 経由で送信する。
func (s *RealSender) SendInquiryReceipt(ctx context.Context, inquiry *domain.Inquiry, snippet string) error {
	bodyText, err := s.renderBody(inquiry, snippet)
	if err != nil {
		return err
	}
	subject, err := s.renderSubject(inquiry)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": inquiry.ReplyEmail}}},
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
	defer func() { _ = resp.Body.Close() }()

	// SendGrid は成功時 202 Accepted、失敗時は 4xx/5xx + JSON エラーボディを返す。
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid api error: status=%d", resp.StatusCode)
	}
	return nil
}

// renderBody は本文テンプレートを評価する。
func (s *RealSender) renderBody(inquiry *domain.Inquiry, snippet string) (string, error) {
	var buf bytes.Buffer
	if err := s.bodyTmpl.Execute(&buf, buildTemplateData(inquiry, snippet)); err != nil {
		return "", fmt.Errorf("render receipt body: %w", err)
	}
	return buf.String(), nil
}

// renderSubject は件名テンプレートを評価する。
func (s *RealSender) renderSubject(inquiry *domain.Inquiry) (string, error) {
	var buf bytes.Buffer
	if err := s.subjectTmpl.Execute(&buf, buildTemplateData(inquiry, "")); err != nil {
		return "", fmt.Errorf("render receipt subject: %w", err)
	}
	return buf.String(), nil
}

// buildTemplateData はテンプレート評価用のビューモデルを組み立てる。
func buildTemplateData(inquiry *domain.Inquiry, snippet string) map[string]any {
	return map[string]any{
		"InquiryID":   inquiry.InquiryID,
		"Title":       inquiry.Title,
		"ReplyEmail":  inquiry.ReplyEmail,
		"CreatedAt":   inquiry.CreatedAt,
		"BodySnippet": snippet,
	}
}
