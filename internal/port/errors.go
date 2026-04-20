// Package port は service 層が依存する抽象インターフェースの集合。
// 具体実装 (postgres / slack / sendgrid 等) は adapter / repository 層に置く。
package port

import "errors"

// ErrNotFound は DB 上に対象行が存在しないことを示すセンチネル。
// handler 層で 404 に変換される。
var ErrNotFound = errors.New("port: not found")
