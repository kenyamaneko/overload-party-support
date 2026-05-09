package domain

import "time"

// Inquiry は問い合わせの内部モデル (`support.inquiries` 1 行)。
type Inquiry struct {
	InquiryID    int64
	Title        string
	Body         string
	ReplyEmail   string
	Status       string
	InternalNote *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
