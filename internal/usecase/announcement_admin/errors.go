package announcementadmin

import "errors"

// ErrNotFound はお知らせが存在しない場合に返す。
var ErrNotFound = errors.New("announcement_admin: not found")

// ErrInvalidType はお知らせ type が許容値外の場合に返す。
var ErrInvalidType = errors.New("announcement_admin: invalid type")

// ErrUnsupportedLang は翻訳 upsert で対応外の lang が指定された場合に返す。
var ErrUnsupportedLang = errors.New("announcement_admin: unsupported lang")

// ErrInvalidField は title / body 等のフィールド長 / 空チェック違反時に返す。
var ErrInvalidField = errors.New("announcement_admin: invalid field")

// ErrInvalidStatusFilter は一覧 API の status クエリ値が許容外の場合に返す。
var ErrInvalidStatusFilter = errors.New("announcement_admin: invalid status filter")
