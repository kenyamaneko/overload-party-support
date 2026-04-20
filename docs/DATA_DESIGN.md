# support スキーマ - データ設計

> **DDL の SSoT:** `db/schema.sql`

## 設計概要

support スキーマはお知らせコンテンツと問い合わせ履歴を管理する。他スキーマへのクロススキーマ参照は持たず、Pub/Sub publish も行わない自己完結型。

---

## テーブル構成

### announcements

お知らせ本体。ロケール非依存の属性（種別・公開期間）を持つ。

- **PK:** `announcement_id` (BIGINT IDENTITY)
- **TRIGGER:** `updated_at` 自動更新

<!-- BEGIN GENERATED: announcements -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `announcement_id` | BIGINT (IDENTITY) | No | 自動採番 |
| `type` | VARCHAR(20) | No | 種別 (info / maintenance / event / update) |
| `published_at` | TIMESTAMPTZ | Yes | 公開開始日時。NULL は下書き状態、未来日時は予約公開 |
| `expires_at` | TIMESTAMPTZ | Yes | 公開終了日時。NULL は無期限公開 |
| `created_at` | TIMESTAMPTZ | No | レコード作成日時 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時（trigger で自動更新） |
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
| `announcement_id` | BIGINT | No | 親お知らせ |
| `lang` | VARCHAR(10) | No | 言語コード (`ja` / `en` 等。SSoT は support 内の定数) |
| `title` | VARCHAR(200) | No | タイトル |
| `body` | TEXT | No | 本文（長さ上限なし） |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: announcement_translations -->

**設計判断:**
- 本体と翻訳を正規化することで、翻訳の後出し追加や差し替えを本体更新なしで行える
- 親の CASCADE DELETE によりお知らせ削除時に翻訳行も自動で削除され、孤児行が残らない
- 一覧取得の JOIN コストは 1 テーブルぶん増えるが、お知らせ件数は小規模（数百件オーダー）を想定しており実害なし
- `lang` の列挙は ENUM ではなく VARCHAR を採用。support 側のコードで許容値を強制し、未知値はリクエストをエラー化（`ErrUnsupportedLang`）する。言語追加時に DDL 変更を伴わない
- 指定 `lang` の翻訳が存在しないお知らせは一覧から除外・詳細で 404。フォールバックは行わない（CLAUDE.md「デフォルト値へのフォールバックを行わない」に従う）

### inquiries

問い合わせ本体と対応ステータス・対応メモを一体で持つ。

- **PK:** `inquiry_id` (BIGINT IDENTITY)
- **CHECK:** `status IN ('new', 'in_progress', 'closed')`
- **TRIGGER:** `updated_at` 自動更新

<!-- BEGIN GENERATED: inquiries -->
| カラム名 | 型 | Nullable | 説明 |
|---|---|---|---|
| `inquiry_id` | BIGINT (IDENTITY) | No | 自動採番 |
| `title` | VARCHAR(100) | No | 件名 |
| `body` | VARCHAR(4000) | No | 本文 |
| `reply_email` | VARCHAR(255) | No | 返信先メールアドレス |
| `status` | VARCHAR(20) | No | new / in_progress / closed |
| `internal_note` | TEXT | Yes | 運営向け対応メモ（プレイヤーからは参照不可） |
| `created_at` | TIMESTAMPTZ | No | 受付日時 |
| `updated_at` | TIMESTAMPTZ | No | 更新日時 |
<!-- END GENERATED: inquiries -->

**設計判断:**
- 問い合わせと対応履歴を別テーブルにしない。1 件の問い合わせに対する履歴は状態遷移のみで、1:N の詳細対応ログは持たない。詳細な対応経緯は `internal_note` に自由記述するか Slack 側で管理する前提
- `status` の許容値は CHECK 制約で DB 層でも強制。アプリ側のバグで不正値が混入する経路を構造的に塞ぐ
- `reply_email` にインデックスは張らない。同一メールアドレスでの過去履歴検索要件が生じた段階で追加検討

---

## テーブル間リレーション

```
announcements (PK: announcement_id)
  │
  └── 1:N ── announcement_translations (PK: announcement_id, lang)
              (FK: announcement_id, CASCADE DELETE)

inquiries (PK: inquiry_id)
  （単独テーブル、FK なし）
```

Support はクロススキーマ参照を持たず、他スキーマ由来の read model も保持しない。

---

## インデックス戦略

| テーブル | インデックス | 用途 |
|---|---|---|
| `announcements` | partial `(published_at DESC, announcement_id DESC) WHERE published_at IS NOT NULL` | 公開 API 一覧取得（下書きを索引に載せない） |
| `inquiries` | `(status, updated_at DESC)` | Slack での未対応・対応中一覧取得 |

お知らせ公開 API のクエリは `published_at IS NOT NULL AND published_at <= now AND (expires_at IS NULL OR now < expires_at)` でフィルタするため、部分インデックスで下書き行 (`published_at IS NULL`) を除外する。管理 UI 向け一覧（下書きを含む全件表示）は別パスで、PK 走査 + 全件スキャンで十分。

一覧の並び順に `updated_at DESC` を採用する理由: 受付直後は `created_at = updated_at` なので新着も先頭に出る。古い問い合わせでもステータス遷移や対応メモ更新があれば上位に浮上し、運営が直近動かした案件を見失わない。`created_at` 基準だと何日も前に受けた案件が対応中でも下に沈む。

他のクエリパスは PK 走査で十分。問い合わせ件数は数千件 / 月オーダーを想定し、全走査でも許容できる前提。スケールが変われば `reply_email` / `created_at` 単独インデックス追加を再検討する。
