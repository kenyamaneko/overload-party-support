package apisupport

import "time"

// Announcement はお知らせ本体のドメインモデル (`support.announcements` 1 行)。
// published_at IS NULL = 下書き状態 (FEATURE_SPEC §4.2 / §6.3)。
type Announcement struct {
	AnnouncementID int64
	Type           Type
	PublishedAt    *time.Time // NULL: 下書き
	ExpiresAt      *time.Time // NULL: 無期限
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Translation はお知らせの言語別翻訳 (`support.announcement_translations` 1 行)。
type Translation struct {
	AnnouncementID int64
	Lang           string
	Title          string
	Body           string
	UpdatedAt      time.Time
}

// AnnouncementWithTranslations は管理 UI が記事編集画面で扱う、本体 + 全言語翻訳のペア。
type AnnouncementWithTranslations struct {
	Announcement Announcement
	Translations []Translation
}

// Inquiry は問い合わせのドメインモデル (`support.inquiries` 1 行)。
type Inquiry struct {
	InquiryID    int64
	Title        string
	Body         string
	ReplyEmail   string
	Status       Status
	InternalNote *string // NULL: メモなし
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
