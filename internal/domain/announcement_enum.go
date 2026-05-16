package domain

// Type はお知らせ種別 (FEATURE_SPEC §2)。本パッケージが SSoT。
const (
	TypeInfo        = "info"
	TypeMaintenance = "maintenance"
	TypeEvent       = "event"
	TypeUpdate      = "update"
)

// Lang は対応言語 (FEATURE_SPEC §3)。本パッケージが SSoT。
const (
	LangJa = "ja"
	LangEn = "en"
)

// AnnouncementState は published_at / expires_at から domain が導出する state (FEATURE_SPEC §6.3)。
// domain 内部完結 (wire には流出しない)。
const (
	StateDraft     = "draft"
	StateScheduled = "scheduled"
	StatePublished = "published"
	StateExpired   = "expired"
)
