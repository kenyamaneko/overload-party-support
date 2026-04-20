package apisupport

import "time"

// Status は問い合わせ対応ステータス enum (FEATURE_SPEC §8.1)。
type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// Statuses は status の許容値一覧。
var Statuses = []Status{StatusNew, StatusInProgress, StatusClosed}

// IsSupportedStatus は status 文字列が許容値に含まれるか判定する。
func IsSupportedStatus(s string) bool {
	for _, v := range Statuses {
		if Status(s) == v {
			return true
		}
	}
	return false
}

// SubmitInquiryRequest は `POST /api/v1/inquiries` の入力 (FEATURE_SPEC §7.1)。
type SubmitInquiryRequest struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	ReplyEmail string `json:"reply_email"`
}

// SubmitInquiryResponse は `POST /api/v1/inquiries` の出力。
type SubmitInquiryResponse struct {
	InquiryID int64 `json:"inquiry_id"`
}

// InquirySummary は `GET /internal/v1/inquiries` のレスポンス要素 (FEATURE_SPEC §8.3)。
type InquirySummary struct {
	InquiryID  int64     `json:"inquiry_id"`
	Title      string    `json:"title"`
	ReplyEmail string    `json:"reply_email"`
	Status     Status    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// InquiryListResponse は `GET /internal/v1/inquiries` のトップレベル JSON。
type InquiryListResponse struct {
	Inquiries []InquirySummary `json:"inquiries"`
}

// InquiryDetail は `GET /internal/v1/inquiries/:id` のレスポンス。internal_note を含む。
type InquiryDetail struct {
	InquiryID    int64     `json:"inquiry_id"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	ReplyEmail   string    `json:"reply_email"`
	Status       Status    `json:"status"`
	InternalNote *string   `json:"internal_note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpdateInquiryStatusRequest は `POST /internal/v1/inquiries/:id/status` の入力。
type UpdateInquiryStatusRequest struct {
	Status Status `json:"status"`
}

// UpdateInquiryNoteRequest は `POST /internal/v1/inquiries/:id/note` の入力。
// null / 空文字はメモ削除として扱う。
type UpdateInquiryNoteRequest struct {
	InternalNote *string `json:"internal_note"`
}
