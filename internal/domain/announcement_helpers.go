package domain

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
