package domain

import "time"

// Types はお知らせ種別の許容値 SSoT。
var Types = []string{TypeInfo, TypeMaintenance, TypeEvent, TypeUpdate}

// IsSupportedType は type 値が許容範囲にあるか判定する。
func IsSupportedType(t string) bool {
	for _, v := range Types {
		if t == v {
			return true
		}
	}
	return false
}

// SupportedLangs は対応言語の許容値 SSoT。
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

// DeriveState は本体属性から state を導出する。
func DeriveState(a Announcement, now time.Time) string {
	// PublishedAt 未設定なら ExpiresAt の値に依らず Draft。最初に判定して state 判定の排他性を担保する。
	if a.PublishedAt == nil {
		return StateDraft
	}
	if a.PublishedAt.After(now) {
		return StateScheduled
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return StateExpired
	}
	return StatePublished
}
