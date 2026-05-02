// Package port は service 層が依存する抽象インターフェースの集合。
package port

import "errors"

// ErrNotFound は DB 上に対象行が存在しないことを示すセンチネル。
var ErrNotFound = errors.New("port: not found")
