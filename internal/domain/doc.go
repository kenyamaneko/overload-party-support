// Package domain は support サービスの内部限定ドメインモデル (entity / enum / 派生 state) を集約する。
// ここに置く型は「DB 行に対応する内部モデル」「業務上の enum」「外部 API には露出しない state」など、
// 外部 (gateway 等) には公開しない概念のみ。wire format は packages/api-support 側で定義され、
// delivery 層 (handler) が domain → wire の詰め替えを担う。
//
// 型と enum 定数は data/models.yaml を SSoT とし、scripts/generate_types.py が *_gen.go を生成する。
// 生成対象外の許容値スライス / IsSupported* helper は本パッケージ内の手書きファイルで提供する。
package domain
