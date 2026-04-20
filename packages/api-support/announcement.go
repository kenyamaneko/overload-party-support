package apisupport

import "time"

// Type はお知らせの種別 enum (FEATURE_SPEC §2)。
type Type string

const (
	TypeInfo        Type = "info"
	TypeMaintenance Type = "maintenance"
	TypeEvent       Type = "event"
	TypeUpdate      Type = "update"
)

// Types はサポート種別の許容値一覧。
var Types = []Type{TypeInfo, TypeMaintenance, TypeEvent, TypeUpdate}

// IsSupportedType は type 値が許容範囲にあるか判定する。
func IsSupportedType(t string) bool {
	for _, v := range Types {
		if Type(t) == v {
			return true
		}
	}
	return false
}

// 対応言語 (FEATURE_SPEC §3)。
const (
	LangJa = "ja"
	LangEn = "en"
)

// SupportedLangs は許容される言語コードの SSoT。
var SupportedLangs = []string{LangJa, LangEn}

// IsSupportedLang は指定 lang が対応言語集合に含まれるかを返す。
func IsSupportedLang(lang string) bool {
	for _, v := range SupportedLangs {
		if lang == v {
			return true
		}
	}
	return false
}

// AnnouncementSummary は `GET /internal/v1/announcements` のレスポンス要素 (FEATURE_SPEC §4)。
// expires_at / body はクライアントに露出させない。
type AnnouncementSummary struct {
	AnnouncementID int64     `json:"announcement_id"`
	Type           Type      `json:"type"`
	Title          string    `json:"title"`
	PublishedAt    time.Time `json:"published_at"`
}

// AnnouncementListResponse は `GET /internal/v1/announcements` のトップレベル JSON。
type AnnouncementListResponse struct {
	Announcements []AnnouncementSummary `json:"announcements"`
}

// AnnouncementDetail は `GET /internal/v1/announcements/:id` のレスポンス (FEATURE_SPEC §5)。
// published_at は下書きの場合 null、expires_at はクライアントに露出させない。
type AnnouncementDetail struct {
	AnnouncementID int64      `json:"announcement_id"`
	Type           Type       `json:"type"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	PublishedAt    *time.Time `json:"published_at"`
}
