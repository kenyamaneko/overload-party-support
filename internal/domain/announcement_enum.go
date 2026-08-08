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
