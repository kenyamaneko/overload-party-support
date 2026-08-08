# support スキーマ - データ設計

> **DDL の SSoT:** `db/schema.sql`

## 設計概要

support スキーマはお知らせコンテンツを管理する。他スキーマへのクロススキーマ参照は持たず、Pub/Sub publish も行わない自己完結型。

---

## テーブル構成

### announcements

お知らせ本体。ロケール非依存の属性（種別・公開期間）を持つ。

- **PK:** `announcement_id` (BIGINT IDENTITY)
- **TRIGGER:** `updated_at` 自動更新

<!-- BEGIN GENERATED: announcements -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `announcement_id` | BIGINT (IDENTITY) | No |  |
| `type` | VARCHAR(20) | No | info / maintenance / event / update |
| `published_at` | TIMESTAMPTZ | Yes | NULL: 下書き, 未来日時: 予約公開, <=now: 公開中 |
| `expires_at` | TIMESTAMPTZ | Yes | NULL: 無期限公開 |
| `created_at` | TIMESTAMPTZ | No | レコード作成日時 |
| `updated_at` | TIMESTAMPTZ | No | 最終更新日時 (trigger で自動更新) |
<!-- END GENERATED: announcements -->

**設計判断:**
- 公開状態を `published_at` と `expires_at` の組だけで表現し、専用の `status` 列を持たない。`published_at IS NULL` = 下書き、`published_at > now` = 予約公開、`published_at <= now < expires_at` = 公開中、`expires_at <= now` = 期限切れ。状態遷移は UPDATE 1 つで完結する
- `created_at` と `published_at` を分離することで、ドラフト保存（`published_at = NULL`）→ 予約公開（`published_at = 未来`）→ 即時公開（`published_at = now`）の遷移が本体 UPDATE だけで行える
- `expires_at` を NULL 許容にしているのは、常設の利用規約リンク等、期限の無い告知を扱うため

### announcement_translations

お知らせの多言語コンテンツ。ロケール単位で行を持つ。

- **PK:** `(announcement_id, lang)`
- **FK:** `announcement_id` → `announcements.announcement_id` (CASCADE DELETE)

<!-- BEGIN GENERATED: announcement_translations -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `announcement_id` | BIGINT | No |  |
| `lang` | VARCHAR(10) | No | ja / en 等 (許容値は support 側の定数) |
| `title` | VARCHAR(200) | No |  |
| `body` | TEXT | No |  |
| `updated_at` | TIMESTAMPTZ | No | trigger で自動更新 |
<!-- END GENERATED: announcement_translations -->

**設計判断:**
- 本体と翻訳を正規化することで、翻訳の後出し追加や差し替えを本体更新なしで行える
- 親の CASCADE DELETE によりお知らせ削除時に翻訳行も自動で削除され、孤児行が残らない
- 一覧取得の JOIN コストは 1 テーブルぶん増えるが、お知らせ件数は小規模（数百件オーダー）を想定しており実害なし
- `lang` の列挙は ENUM ではなく VARCHAR を採用。support 側のコードで許容値を強制し、未知値はリクエストをエラー化（`ErrUnsupportedLang`）する。言語追加時に DDL 変更を伴わない
- 指定 `lang` の翻訳が存在しないお知らせは一覧から除外・詳細で 404。フォールバックは行わない（CLAUDE.md「デフォルト値へのフォールバックを行わない」に従う）

---

## テーブル間リレーション

```
announcements (PK: announcement_id)
  │
  └── 1:N ── announcement_translations (PK: announcement_id, lang)
              (FK: announcement_id, CASCADE DELETE)
```

Support はクロススキーマ参照を持たず、他スキーマ由来の read model も保持しない。

---

## インデックス戦略

| テーブル | インデックス | 用途 |
|---|---|---|
| `announcements` | partial `(published_at DESC, announcement_id DESC) WHERE published_at IS NOT NULL` | 公開 API 一覧取得（下書きを索引に載せない） |

お知らせ公開 API のクエリは `published_at IS NOT NULL AND published_at <= now AND (expires_at IS NULL OR now < expires_at)` でフィルタするため、部分インデックスで下書き行 (`published_at IS NULL`) を除外する。

他のクエリパスは PK 走査で十分。お知らせ件数は小規模（数百件オーダー）を想定し、全走査でも許容できる前提。
