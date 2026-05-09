# Support 機能仕様書

このドキュメントは support サービスがビジネス要件として満たすべき振る舞いを定義する。実装方法ではなく **何を保証するか** を記述する。テストはこの仕様に従っていることを確認する観点で書く。

関連ドキュメント:
- 内部動作・配線・本番運用設定: [ARCHITECTURE.md](ARCHITECTURE.md)
- HTTP エンドポイント契約: [../data/openapi.yaml](../data/openapi.yaml)
- DB スキーマ: [DATA_DESIGN.md](DATA_DESIGN.md)

---

## 1. サービス責務

support は以下の機能ドメインを所有する。

| 機能 | 主要な責務 |
|---|---|
| お知らせ一覧取得 | 公開中のお知らせを多言語対応で返す |
| お知らせ詳細取得 | 単一のお知らせを多言語対応で返す |
| お知らせ管理 UI | 運用者がお知らせの作成・編集・翻訳追加・公開制御を行う Web 画面 |
| 問い合わせ受付 | 外部からの問い合わせを受理し Slack 通知 + 受付確認メールを送る |
| 問い合わせステータス管理 | 運営（Slack 連携）による対応ステータス・対応メモの更新 |

support は **support スキーマの DB 行を唯一の真実とする**。お知らせのコンテンツは、以前 gateway が保持していた静的 JSON (`data/announcements.json`) を置き換える支配的情報源となる。

### 1.1 公開境界

| 経路 | ポート | 用途 | 認証 |
|---|---|---|---|
| gateway 経由 `GET /api/v1/announcements`, `/api/v1/announcements/:id` | `:9009` | プレイヤー・クライアント向け読み取り | 不要（gateway 側も公開ルート） |
| 管理 UI `GET /admin/*` / `POST /admin/*` | `:9109` | 運用者ブラウザによるお知らせ CRUD / 問い合わせ管理 | IAP (Identity-Aware Proxy) |
| 外部直通 `POST /api/v1/inquiries` | `:9209` | 問い合わせフォーム（Web）からの直アクセス | 不要（CORS で Origin 制限） |

---

## 2. お知らせ種別

お知らせは `Announcement.Type` で以下に分かれる。種別はクライアント側でのアイコン・並び順制御のヒントとして扱う。

| Type | 意味 |
|---|---|
| `info` | 一般的なお知らせ |
| `maintenance` | メンテナンス告知 |
| `event` | イベント告知 |
| `update` | アップデート告知 |

未知の type をクライアントが受け取っても表示は継続する（将来の type 追加を互換に受け入れる）。

---

## 3. 多言語対応

各お知らせは複数言語の翻訳 (`title`, `body`) を持つ。MVP 対応言語は `ja` と `en` の 2 種。許容値は support サービス内の定数で管理し、未知の値をリクエストで受けた場合はエラーとする。

### 3.1 データ構造（正規化）

お知らせ本体 (`announcements`) と翻訳 (`announcement_translations`) を別テーブルに正規化する。本体には言語非依存な属性のみを持つ（`type` / `published_at` / `expires_at` / `created_at` など）。`title` / `body` は翻訳テーブルで `(announcement_id, lang)` の複合キーで管理する。

これにより、

- 同一お知らせに対して後から別言語の翻訳を追加する
- 特定言語の翻訳だけ差し替える

といった操作が本体行を触らずに行える。詳細なスキーマは [DATA_DESIGN.md](DATA_DESIGN.md) で定義する。

### 3.2 言語解決（フォールバックしない）

1. 公開 API は `?lang=<code>` を **必須** クエリで受ける。欠落時は `ErrLangRequired`（400）
2. 対応外の言語コードは `ErrUnsupportedLang`（400）
3. 指定言語の翻訳が存在しない記事は、**一覧では除外・詳細では 404**
4. **フォールバックは行わない**（CLAUDE.md「デフォルト値へのフォールバックを行わない」に従う）

「運営はまず ja 翻訳を用意してから公開する」という運用契約で、翻訳未整備の記事が露出する事態を防ぐ。ja を公開した状態で en 未整備の場合、ja ユーザには表示されるが en ユーザには見えない。運営がこれを検知し en 翻訳を追加するまでが運用者責務。

---

## 4. お知らせ一覧取得 (`GetAnnouncements`)

**入力**: `lang`（必須）
**出力**: `[]AnnouncementSummary`

### 4.1 仕様

1. `lang` 必須。欠落は `ErrLangRequired`、対応外値は `ErrUnsupportedLang`
2. `published_at IS NOT NULL AND published_at <= now AND (expires_at IS NULL OR now < expires_at)` を満たす行のみを返す（`now` はサーバ時刻）
3. `published_at IS NULL` の下書き行は除外する（§6.3）
4. `expires_at IS NULL` のお知らせは無期限公開として扱う
5. `published_at` を将来日時にすることで予約公開になる（現在時刻が `published_at` 未満のお知らせは返さない）
6. **指定 `lang` の翻訳が存在しない記事は除外する**（§3.2 フォールバックなし方針）
7. 並び順は `published_at DESC, announcement_id DESC`
8. ページング・絞り込みは行わない（全件返却）
9. 各行は翻訳の `title`、本体の `type` / `published_at` を含む（`body` / `expires_at` は含まない）

副作用なし。`expires_at` はサーバ側の公開期間フィルタでのみ使用し、クライアントには露出しない。

### 4.2 `created_at` / `published_at` / `NULL` の三者の関係

お知らせの状態は `created_at` と `published_at` の組で表現する:

- 下書き: `created_at = 作成時刻`, `published_at = NULL`
- 予約公開: `created_at = 作成時刻`, `published_at = 未来`
- 公開中: `created_at = 作成時刻`, `published_at <= now`

これにより、ドラフト作成と公開を別操作にしつつも、`status` 列を持たず DDL を簡潔に保てる。詳細な状態遷移は §6.3。

---

## 5. お知らせ詳細取得 (`GetAnnouncement`)

**入力**: `announcementID`, `lang`（必須）
**出力**: `AnnouncementDetail`, `error`

### 5.1 仕様

1. `lang` 必須。欠落は `ErrLangRequired`、対応外値は `ErrUnsupportedLang`
2. 以下のいずれかなら `ErrAnnouncementNotFound`（404）:
   - `announcementID` で記事行が存在しない
   - 指定 `lang` の翻訳行が存在しない
3. **公開期間外の行も返す**（§5.2 参照）。ただし `published_at IS NULL` の下書きも返す点に留意（ID を知る運営のプレビュー用途）
4. `body` を含む完全なレコードを返す（ただし `expires_at` はレスポンスに含めない）

副作用なし。

### 5.2 公開期間外と下書きも返す理由

一覧は「今プレイヤーに見せるべきもの」を返すが、詳細は URL 共有・プッシュ通知のリンクから到達するため、期間外でも内容を表示できる方がユーザー体験上望ましい。`expires_at` はクライアントに露出しないため「期限切れ」表示は行わない。期限切れのお知らせでもコンテンツそのものを素直に表示し、必要に応じてお知らせ本文内で自己完結的に「このイベントは終了しました」等を記述する運用に寄せる。

下書き (`published_at IS NULL`) や予約公開 (`published_at > now`) に関しても詳細は返すが、ID を知っているのは運営のみなので事実上内部プレビュー用途になる。

---

## 6. お知らせ管理 UI

運用者がお知らせの作成・編集・翻訳追加・公開制御を行う Web 画面を support サービス本体が提供する。FE 成果物を独立デプロイせず、support バイナリ内の `html/template` + HTMX で動的レンダリングする（news サービスと同じパターン）。

### 6.1 画面構成

| 画面 / 操作 | パス | 機能 |
|---|---|---|
| お知らせ一覧 | `GET /admin/announcements` | 状態フィルタ付きの全お知らせ一覧（下書き・予約公開・公開中・期限切れを状態バッジで表示） |
| 新規作成フォーム | `GET /admin/announcements/new` | 新規作成フォーム（type, published_at, expires_at, ja 翻訳を同一フォームで入力） |
| 新規作成 (POST) | `POST /admin/announcements` | §6.4 仕様で本体 + ja 翻訳を INSERT |
| 編集画面 | `GET /admin/announcements/:announcementId` | 編集画面（本体属性 + 言語タブ付き翻訳編集） |
| 本体更新 | `POST /admin/announcements/:announcementId` | `type` / `published_at` / `expires_at` を更新 |
| 削除 | `POST /admin/announcements/:announcementId/delete` | 記事行を削除（翻訳は FK CASCADE） |
| 翻訳 upsert | `POST /admin/announcements/:announcementId/translations/:lang` | §6.5 仕様で翻訳を INSERT / UPDATE |

一覧画面には「+新規作成」ボタン、各行には編集・削除への動線を置く。編集画面の言語タブは `ja` / `en` の 2 タブ構成で、未作成の言語タブには空フォームを表示する。

### 6.2 アクセス制御

- 管理 UI は **IAP (Identity-Aware Proxy) 背後にのみ露出** する（`:9109` ポート、公開 API とは別ポート・別 Ingress）
- 認証は IAP が完結させる。support 側の責務は `X-Goog-Authenticated-User-Email` ヘッダの存在確認のみ
- ヘッダが無いリクエストは middleware で `ErrMissingIAPHeader`（401）
- 許可メールリストの管理は IAP 側で行う（support のデプロイ不要）
- `ENV=local` では IAP middleware はパススルーする（local 開発で IAP を立てないため）

### 6.3 公開状態の表現（下書きは `published_at = NULL`）

お知らせは `status` 列を持たず、`published_at` と `expires_at` の組で公開状態を表現する:

| published_at | expires_at | 状態 | 公開 API での扱い |
|---|---|---|---|
| NULL | (any) | 下書き | 一覧除外・詳細は ID 知る運営のみ可視 |
| `> now` | (any) | 予約公開待ち | 一覧除外・詳細は同上 |
| `<= now` | NULL または `now < expires_at` | 公開中 | 一覧に露出・詳細で返却 |
| `<= now` | `<= now` | 期限切れ | 一覧除外・詳細では返却（§5.2） |

下書き → 予約公開 → 公開中 → 期限切れの状態遷移は `published_at` / `expires_at` の更新だけで実現する。明示的な `publish` / `unpublish` API は持たない（運用上の混乱を招くため）。

### 6.4 本体 CRUD

| 操作 | 変更対象 | 入力 | 備考 |
|---|---|---|---|
| 新規作成 (`CreateAnnouncement`) | `announcements` 1 行 + ja 翻訳 1 行 | `type`, `published_at?`, `expires_at?`, `ja_title`, `ja_body` | 本体と ja 翻訳を **同一トランザクション** で INSERT |
| 本体更新 (`UpdateAnnouncement`) | `announcements.type`, `published_at`, `expires_at` | 同上 | 翻訳は別エンドポイント（§6.5） |
| 削除 (`DeleteAnnouncement`) | `announcements` 1 行（翻訳は FK CASCADE） | `announcementID` | |

新規作成時に ja 翻訳を必須にするのは、§3.2 の運用契約「まず ja 翻訳を用意してから公開する」を DB レベルで支援するため。翻訳 0 件のお知らせが作られない状態を構造的に保証する。

### 6.5 翻訳 upsert

| 操作 | 変更対象 | 入力 |
|---|---|---|
| 翻訳 upsert (`UpsertAnnouncementTranslation`) | `announcement_translations` の `title` / `body` | `announcementID`, `lang`, `title`, `body` |

既存なら UPDATE、未存在なら INSERT。ja 翻訳も en 翻訳も同じ口で更新する。記事本体の `updated_at` は動かさない（翻訳編集と本体更新を別監査対象として分離）。

### 6.6 バリデーション

- `type`: §2 の 4 種のいずれか。未知値は `ErrInvalidAnnouncementType`（400）
- `lang`: §3 の対応言語のいずれか。未知値は `ErrUnsupportedLang`（400）
- `title`: 1 文字以上 200 文字以下。範囲外は `ErrInvalidField`（400）
- `body`: 1 文字以上（上限なし）。空文字は `ErrInvalidField`
- `published_at`: TIMESTAMPTZ または空（NULL）。過去日時も許容（即時公開として `now()` を明示的に設定する運用）
- `expires_at`: TIMESTAMPTZ または空（NULL）。`published_at` との前後関係は DB レベルで強制しない（運用判断を許容）

### 6.7 同時編集

楽観ロックや競合検出は行わない。複数の運用者が同じお知らせを同時編集した場合は後勝ちで `updated_at` が最後の更新時刻になる。運用者数が少数前提（news と同じ判断）。

副作用（Pub/Sub 等）は発生しない。

---

## 7. 問い合わせ受付 (`SubmitInquiry`)

**入力**: `title`, `body`, `replyEmail`
**出力**: `error`

問い合わせ本体は多言語対応しない（言語不問）。受付確認メールのテンプレートは日本語固定。

### 7.1 バリデーションと副作用順序（fail-fast）

以下の順で処理し、失敗時点で即 return する。順序自体が仕様。

1. **必須フィールド**: `title`, `body`, `replyEmail` のいずれかが空なら `ErrInvalidInquiry`
2. **長さ上限**: `title` は 100 文字、`body` は 4000 文字を超えたら `ErrInvalidInquiry`（自動切り詰めは行わない）
3. **email 形式**: `replyEmail` が RFC 5322 準拠で解釈できなければ `ErrInvalidEmail`
4. **DB 書き込み**: `support.inquiries` に `status = new` で INSERT
5. **Slack 通知**: §9.1 に従い運営チャンネルへ通知を送る
6. **受付確認メール送信**: §7.2 に従い SendGrid 経由で `replyEmail` 宛に自動返信する

### 7.2 受付確認メール (auto-reply)

DB 書き込み成功後、SendGrid 経由で `replyEmail` 宛に以下を含む受付確認メールを送る:

- 問い合わせ ID
- 受付日時
- 件名 (`title`) と本文 (`body`) のサマリ
- 「担当者から改めてご連絡します」旨の定型文

テンプレートは日本語固定。件名・本文は support 内の `html/template` で組み立て、SendGrid Go SDK の `Mail` オブジェクトに渡す（プロバイダ差し替え時は `port.EmailSender` の adapter を付け替えるだけで済む）。from アドレス・差出人名は環境変数で外出し可能とする。

### 7.3 副作用失敗時の扱い

Slack 通知と SendGrid 送信は **DB COMMIT 後** に §7.1 の順序で逐次実行する。いずれかが失敗した時点で即 return し、呼び出し元にエラーを返す（CLAUDE.md「握りつぶし禁止」）。

- Slack 通知が成功し SendGrid が失敗した場合: 運営側には ticket が見えるので、運営が Slack 経由で再送指示 or 手動返信する
- Slack 通知が失敗した場合: DB には行が残るため、§8.3 の内部 API で未通知分を発見・救済できる

フォーム側のリトライは §7.4 の通り冪等性を保証しない。

### 7.4 冪等性の非保証

問い合わせには purchaseToken のような外部識別子が存在しないため、**完全な冪等性は保証しない**。同一内容の問い合わせが短時間に複数登録された場合は運営側で Slack 上手動でマージする運用とする。フォーム側の二重送信防止はクライアント責務。

### 7.5 認証と公開面

このエンドポイントは **gateway を経由しない外部から呼ばれる**（問い合わせフォームからの直接アクセス）。認証は掛けない。ゲストユーザーも問い合わせ可能。

レート制限・CAPTCHA 等のスパム対策は優先度低の将来課題とし、現時点では仕様に含めない。ただし auto-reply を実装する以上、攻撃者が第三者のメールアドレスを `replyEmail` に詐称すると受付確認メールが第三者へ届くことになる。内容は無害だが将来の CAPTCHA 検討材料とする。

---

## 8. 問い合わせステータス管理

support は問い合わせの対応ステータスと対応メモを内部で保持し、Slack 連携から照会・更新される。

### 8.1 ステータス遷移

| Status | 意味 | 遷移先 |
|---|---|---|
| `new` | 受付直後、未着手 | `in_progress`, `closed` |
| `in_progress` | 運営が対応中 | `closed` |
| `closed` | クローズ（返信完了・返信不要・スパム等） | ― |

`closed` からの逆遷移は許容せず `ErrInvalidStatusTransition` を返す。再オープンが必要な事案は新規 inquiry として起票する運用とする。

`closed` は「返信完了」「返信不要（スパム等）」の両方を内包する終端状態。両者の区別が事後的に必要になった場合は §8.2 の対応メモに記録するか、専用列 (`closed_reason`) を追加する。

ステータス更新は手動のみ。SendGrid の送信結果や Gmail などから自動遷移させる仕組みは持たない。自動化は外部メール連携コストに見合わないため。

### 8.2 対応メモ (`internal_note`)

問い合わせに 1:1 で紐づく運営向け自由記述メモ。用途:

- 対応の経緯・判断根拠の記録（Slack スレッドを漁らずに済ませる）
- 引き継ぎ時のコンテキスト共有
- `closed` の理由（返信完了 / スパム等）

メモは運営のみが読み書きでき、プレイヤーからは参照されない。個人情報が混入しうるため、DB 暗号化・アクセス制御は [DATA_DESIGN.md](DATA_DESIGN.md) / [ARCHITECTURE.md](ARCHITECTURE.md) で別途定義する。

### 8.3 管理経路

問い合わせの一覧・詳細・ステータス更新・対応メモ更新は **管理 UI (`:9109`)** から運用者が行う。`:9109` は IAP の背後にあり、運用者ブラウザのみがアクセス可能。

一覧は `updated_at DESC` で並べる（`created_at` ではない）。受付直後は `created_at = updated_at` なので新着もトップに出る一方、古い問い合わせでもステータスや対応メモの更新があれば上位に浮上し、運営が直近動かした案件を見失わない。

ステータス更新は `in_progress` / `closed` のみ受け付ける。`new` は受付時にサーバが一度だけセットする初期値で、API 経由では遷移先に指定できない。**返信文面は support では管理しない**（運営が Slack の通知と管理 UI の詳細を元にメールで返信し、その結果を `closed` として記録するのみ）。

### 8.4 プレイヤー向け履歴 API を提供しない

プレイヤーが自身の問い合わせ履歴を閲覧する API は提供しない。返答はすべて `replyEmail` 宛のメールで行う。

---

## 9. Slack 連携

support は問い合わせ受付の **一方向通知** にのみ Slack を使う。Slack workspace・チャンネル ID・bot token は support 自身が Secret Manager から取得し、外部サービスに依存しない。

### 9.1 受付時の通知

`SubmitInquiry` 成功時、運営チャンネル（環境変数で指定）に以下を投稿する:

- 問い合わせ ID
- 件名 (`title`)
- 本文抜粋 (`body` の先頭 200 文字、env で可変)
- 返信先メールアドレス (`replyEmail`)
- 管理 UI へのリンク（`/admin/inquiries/:id`）

通知は read-only で、対応操作は管理 UI 側で行う。support 自身は Slack の interactive payload を解釈しない。

---

## 10. エラーセマンティクス

usecase 層は HTTP ステータスを知らない。エラーはセンチネルとして返し、handler が `errors.Is` ベースの分類関数で transport 層のステータスに変換する（shop の `internal/usecase/<feature>/errors.go` と同じ方針）。

### 10.1 分類

| 分類関数 | 対象エラー | 用途 |
|---|---|---|
| `IsNotFound` | `ErrAnnouncementNotFound` | 404 |
| `IsValidation` | `ErrInvalidInquiry`, `ErrInvalidEmail`, `ErrLangRequired`, `ErrUnsupportedLang`, `ErrInvalidAnnouncementType`, `ErrInvalidField` | 400 |
| `IsUnauthorized` | `ErrMissingIAPHeader` | 401（管理 UI のみ） |

### 10.2 握りつぶし禁止

Slack 通知失敗・SendGrid 送信失敗・DB エラーをログのみで握りつぶしてはならない。すべて呼び出し元に返す（CLAUDE.md「設計思想」参照）。

---

## 11. イベント発行

support は Pub/Sub イベント発行を **行わない**。他サービスとの状態同期が不要（お知らせはプル型、問い合わせは Slack 通知 + 管理 UI で運用が完結する）なため。将来イベント化が必要になった場合は shop と同じ Transactional Outbox パターンを導入する。
