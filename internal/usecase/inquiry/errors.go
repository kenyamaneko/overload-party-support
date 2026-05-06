package inquiry

import "errors"

// ErrInvalidInquiry は必須欠落 / 長さ超過など受付時のバリデーション違反。
var ErrInvalidInquiry = errors.New("inquiry: invalid input")

// ErrInvalidEmail は replyEmail が RFC 5322 準拠で解釈できない場合に返す。
var ErrInvalidEmail = errors.New("inquiry: invalid email")
