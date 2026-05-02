package domain

// Statuses は問い合わせステータスの許容値 SSoT。
var Statuses = []string{StatusNew, StatusInProgress, StatusClosed}

// IsSupportedStatus は status 文字列が許容値に含まれるか判定する。
func IsSupportedStatus(s string) bool {
	for _, v := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}
