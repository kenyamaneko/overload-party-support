package domain

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

// SupportedLangs は対応言語の許容値 SSoT (FEATURE_SPEC §3)。
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
