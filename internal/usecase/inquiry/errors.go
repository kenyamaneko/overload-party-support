package inquiry

import "errors"

// ErrInvalidInquiry は必須欠落 / 長さ超過など受付時のバリデーション違反。
var ErrInvalidInquiry = errors.New("inquiry: invalid input")

// ErrInvalidEmail は replyEmail が RFC 5322 準拠で解釈できない場合に返す。
var ErrInvalidEmail = errors.New("inquiry: invalid email")

// ErrNotFound は inquiry_id で問い合わせが見つからない場合に返す。
var ErrNotFound = errors.New("inquiry: not found")

// ErrInvalidStatusTransition は API 経由で `new` 指定したり closed からの逆遷移等を試みた場合に返す。
var ErrInvalidStatusTransition = errors.New("inquiry: invalid status transition")

// ErrInvalidStatusValue は status クエリ / ボディが未知値の場合に返す。
var ErrInvalidStatusValue = errors.New("inquiry: invalid status value")
