// Package apisupport は support サービスが公開する REST 契約型を提供する。
//
// ここに置く型は「外部（gateway / 問い合わせフォーム / slack-commands）が期待する JSON 形」であり、
// 内部ドメイン型とは別に管理する。内部ロジックは internal/ 配下に閉じる。
package apisupport
