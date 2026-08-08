# Support 機能仕様書

このドキュメントは support サービスがビジネス要件として満たすべき振る舞いを定義する。実装方法ではなく **何を保証するか** を記述する。テストはこの仕様に従っていることを確認する観点で書く。

関連ドキュメント:
- 内部動作・配線・本番運用設定: [ARCHITECTURE.md](ARCHITECTURE.md)
- HTTP エンドポイント契約: [../data/openapi.yaml](../data/openapi.yaml)
- DB スキーマ: [DATA_DESIGN.md](DATA_DESIGN.md)

---

## サービス責務

support は以下の機能ドメインを所有する。

| 機能 | 主要な責務 |
|---|---|
| お知らせ一覧取得 | 公開中のお知らせを多言語対応で返す |
| お知らせ詳細取得 | 単一のお知らせを多言語対応で返す |

support は **support スキーマの DB 行を唯一の真実とする**。お知らせのコンテンツは、以前 gateway が保持していた静的 JSON (`data/announcements.json`) を置き換える支配的情報源となる。

### 公開境界

| 経路 | ポート | 用途 | 認証 |
|---|---|---|---|
| gateway 経由 `GET /api/v1/announcements`, `/api/v1/announcements/:id` | `:9009` | プレイヤー・クライアント向け読み取り | 不要（gateway 側も公開ルート） |

お知らせの登録・更新は運用者が support データベースへ直接 SQL を実行して行う。問い合わせは外部フォームで受ける。

---

## お知らせ種別

お知らせは `Announcement.Type` で以下に分かれる。種別はクライアント側でのアイコン・並び順制御のヒントとして扱う。

| Type | 意味 |
|---|---|
| `info` | 一般的なお知らせ |
| `maintenance` | メンテナンス告知 |
| `event` | イベント告知 |
| `update` | アップデート告知 |

未知の type をクライアントが受け取っても表示は継続する（将来の type 追加を互換に受け入れる）。

---

## 多言語対応

各お知らせは複数言語の翻訳 (`title`, `body`) を持つ。MVP 対応言語は `ja` と `en` の 2 種。許容値は support サービス内の定数で管理し、未知の値をリクエストで受けた場合はエラーとする。

### データ構造（正規化）

お知らせ本体 (`announcements`) と翻訳 (`announcement_translations`) を別テーブルに正規化する。本体には言語非依存な属性のみを持つ（`type` / `published_at` / `expires_at` / `created_at` など）。`title` / `body` は翻訳テーブルで `(announcement_id, lang)` の複合キーで管理する。

これにより、

- 同一お知らせに対して後から別言語の翻訳を追加する
- 特定言語の翻訳だけ差し替える

といった操作が本体行を触らずに行える。詳細なスキーマは [DATA_DESIGN.md](DATA_DESIGN.md) で定義する。

### 言語解決（フォールバックしない）

1. 公開 API は `?lang=<code>` を **必須** クエリで受ける。欠落時は `ErrLangRequired`（400）
2. 対応外の言語コードは `ErrUnsupportedLang`（400）
3. 指定言語の翻訳が存在しない記事は、**一覧では除外・詳細では 404**
4. **フォールバックは行わない**（CLAUDE.md「デフォルト値へのフォールバックを行わない」に従う）

「運営はまず ja 翻訳を用意してから公開する」という運用契約で、翻訳未整備の記事が露出する事態を防ぐ。ja を公開した状態で en 未整備の場合、ja ユーザには表示されるが en ユーザには見えない。運営がこれを検知し en 翻訳を追加するまでが運用者責務。

---

## お知らせ一覧取得 (`GetAnnouncements`)

**入力**: `lang`（必須）
**出力**: `[]AnnouncementSummary`

### 仕様

1. `lang` 必須。欠落は `ErrLangRequired`、対応外値は `ErrUnsupportedLang`
2. `published_at IS NOT NULL AND published_at <= now AND (expires_at IS NULL OR now < expires_at)` を満たす行のみを返す（`now` はサーバ時刻）
3. `published_at IS NULL` の下書き行は除外する（「公開状態の表現」参照）
4. `expires_at IS NULL` のお知らせは無期限公開として扱う
5. `published_at` を将来日時にすることで予約公開になる（現在時刻が `published_at` 未満のお知らせは返さない）
6. **指定 `lang` の翻訳が存在しない記事は除外する**（「言語解決（フォールバックしない）」の方針）
7. 並び順は `published_at DESC, announcement_id DESC`
8. ページング・絞り込みは行わない（全件返却）
9. 各行は翻訳の `title`、本体の `type` / `published_at` を含む（`body` / `expires_at` は含まない）

副作用なし。`expires_at` はサーバ側の公開期間フィルタでのみ使用し、クライアントには露出しない。

### `created_at` / `published_at` / `NULL` の三者の関係

お知らせの状態は `created_at` と `published_at` の組で表現する:

- 下書き: `created_at = 作成時刻`, `published_at = NULL`
- 予約公開: `created_at = 作成時刻`, `published_at = 未来`
- 公開中: `created_at = 作成時刻`, `published_at <= now`

これにより、ドラフト作成と公開を別操作にしつつも、`status` 列を持たず DDL を簡潔に保てる。詳細な状態遷移は「公開状態の表現」を参照。

---

## お知らせ詳細取得 (`GetAnnouncement`)

**入力**: `announcementID`, `lang`（必須）
**出力**: `AnnouncementDetail`, `error`

### 仕様

1. `lang` 必須。欠落は `ErrLangRequired`、対応外値は `ErrUnsupportedLang`
2. 以下のいずれかなら `ErrNotFound`（404）:
   - `announcementID` で記事行が存在しない
   - 指定 `lang` の翻訳行が存在しない
3. **公開期間外の行も返す**（「公開期間外と下書きも返す理由」参照）。ただし `published_at IS NULL` の下書きも返す点に留意（ID を知る運営のプレビュー用途）
4. `body` を含む完全なレコードを返す（ただし `expires_at` はレスポンスに含めない）

副作用なし。

### 公開期間外と下書きも返す理由

一覧は「今プレイヤーに見せるべきもの」を返すが、詳細は URL 共有・プッシュ通知のリンクから到達するため、期間外でも内容を表示できる方がユーザー体験上望ましい。`expires_at` はクライアントに露出しないため「期限切れ」表示は行わない。期限切れのお知らせでもコンテンツそのものを素直に表示し、必要に応じてお知らせ本文内で自己完結的に「このイベントは終了しました」等を記述する運用に寄せる。

下書き (`published_at IS NULL`) や予約公開 (`published_at > now`) に関しても詳細は返すが、ID を知っているのは運営のみなので事実上内部プレビュー用途になる。

---

## 公開状態の表現（下書きは `published_at = NULL`）

お知らせは `status` 列を持たず、`published_at` と `expires_at` の組で公開状態を表現する:

| published_at | expires_at | 状態 | 公開 API での扱い |
|---|---|---|---|
| NULL | (any) | 下書き | 一覧除外・詳細は ID 知る運営のみ可視 |
| `> now` | (any) | 予約公開待ち | 一覧除外・詳細は同上 |
| `<= now` | NULL または `now < expires_at` | 公開中 | 一覧に露出・詳細で返却 |
| `<= now` | `<= now` | 期限切れ | 一覧除外・詳細では返却（「公開期間外と下書きも返す理由」） |

下書き → 予約公開 → 公開中 → 期限切れの状態遷移は `published_at` / `expires_at` の更新だけで実現する。明示的な `publish` / `unpublish` API は持たない（運用上の混乱を招くため）。

---

## エラーセマンティクス

usecase 層は HTTP ステータスを知らない。エラーはセンチネルとして返し、handler が `errors.Is` ベースの分類関数で transport 層のステータスに変換する（shop の `internal/usecase/<feature>/errors.go` と同じ方針）。

### 分類

| 分類関数 | 対象エラー | 用途 |
|---|---|---|
| `IsNotFound` | `ErrNotFound` | 404 |
| `IsValidation` | `ErrLangRequired`, `ErrUnsupportedLang` | 400 |

### 握りつぶし禁止

DB エラーをログのみで握りつぶしてはならない。すべて呼び出し元に返す（CLAUDE.md「設計思想」参照）。

---

## イベント発行

support は Pub/Sub イベント発行を **行わない**。他サービスとの状態同期が不要（お知らせはプル型で配信する）なため。将来イベント化が必要になった場合は shop と同じ Transactional Outbox パターンを導入する。
