package announcement

import "errors"

// ErrLangRequired は lang クエリパラメータが未指定の場合に返す (FEATURE_SPEC §3.2)。
var ErrLangRequired = errors.New("announcement: lang is required")

// ErrUnsupportedLang は対応外の lang が指定された場合に返す。
var ErrUnsupportedLang = errors.New("announcement: unsupported lang")

// ErrNotFound はお知らせが存在しない / 指定 lang の翻訳が存在しない場合に返す。
var ErrNotFound = errors.New("announcement: not found")
