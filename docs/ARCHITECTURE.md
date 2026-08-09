# Support サービス設計

本ドキュメントは **コードを読んでも一見しては分からない設計意図** だけを残す。実装詳細（フロー順序・状態遷移・エラー → HTTP ステータス変換・環境変数一覧）は各ファイルの実装とコメントを一次情報とする。

サービス概要・起動手順は [../README.md](../README.md)、エンドポイントは [../data/openapi.yaml](../data/openapi.yaml)、テーブル定義は [DATA_DESIGN.md](DATA_DESIGN.md) を参照。

## Support の責務境界 (SSoT と書き込み権限)

Support は **お知らせコンテンツ** の single source of truth。`support.announcements` / `support.announcement_translations` への書き込みは support のみが行う。

| ドメイン | 書き手 | 契機 |
|---|---|---|
| お知らせ本体・翻訳 | support | 運用者による登録・更新 |

他サービスとの状態同期は **行わない**（Pub/Sub publish なし）。support は他サービスを直接呼ばない。

## 公開しているポート

support が listen するのは gateway 向け内部 API の 1 ポートだけである。

| ポート | プロトコル | 想定クライアント | 認証 |
|---|---|---|---|
| `:9009` | HTTP JSON | gateway | gateway 経由のみ |

## お知らせ翻訳テーブル分離の意図

お知らせ本体 (`announcements`) と翻訳 (`announcement_translations`) を別テーブルに分けているのは:

- 翻訳の後出し追加（`ja` だけ先に公開し、後日 `en` を追加）を本体更新なしで行いたい
- 特定言語の翻訳だけ差し替えても、他言語の配信・監査ログに影響させない
- `published_at` / `expires_at` などの公開制御を言語と独立に持たせる（言語単位で公開期間を分けるのは UX 上害が多いと判断）

指定 `lang` の翻訳が存在しない記事はフォールバックせず、一覧から除外・詳細で 404 を返す（CLAUDE.md「デフォルト値へのフォールバックを行わない」に従う）。翻訳の未整備を運営に対して「サイレントに成功しない」ことで可視化し、運用者責務で補完させる。

詳細なスキーマは [DATA_DESIGN.md](DATA_DESIGN.md) を参照。

## 下書きは `published_at = NULL` で表現する

お知らせの状態（下書き / 予約公開 / 公開中 / 期限切れ）は `published_at` と `expires_at` の 2 列だけで表現し、`status` 列を持たない:

- 専用の状態遷移 API (`publish` / `unpublish`) を持たず、`published_at` の UPDATE 1 つで公開・非公開が切り替わる
- 予約公開と即時公開の区別が DDL レベルで不要（どちらも `published_at` に timestamp を入れるだけ）
- 公開 API (`:9009`) は `published_at IS NOT NULL AND published_at <= now` でフィルタするため、下書きが誤って露出する経路が構造的に存在しない

`status` 列と状態遷移 API を持つ news とは方針が異なる。news は外部インジェスト由来で「校閲」という明示的な承認フローが必要だが、support のお知らせは運営が手で作るものなので、フロー自体を簡素化できる。

## 運用

### 環境変数 / Secret Manager

環境変数の一覧と必須条件は `internal/config/config.go` が SSoT（起動時に検証、欠ければ即 fail）。

- ローカル開発では `make run` が env を自動注入する

### 外部依存とヘルスチェック

`/health` は DB 接続のみチェックする。
