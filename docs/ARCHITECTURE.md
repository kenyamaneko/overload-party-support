# Support サービス設計

本ドキュメントは **コードを読んでも一見しては分からない設計意図** だけを残す。実装詳細（フロー順序・状態遷移・エラー → HTTP ステータス変換・環境変数一覧）は各ファイルの実装とコメントを一次情報とする。

サービス概要・起動手順は [../README.md](../README.md)、保証すべき振る舞いは [FEATURE_SPEC.md](FEATURE_SPEC.md)、エンドポイントは [API_REFERENCE.md](API_REFERENCE.md)、テーブル定義は [DATA_DESIGN.md](DATA_DESIGN.md) を参照。

## Support の責務境界 (SSoT と書き込み権限)

Support は **お知らせコンテンツ** と **問い合わせ履歴** の single source of truth。`support.*` への書き込みは support のみが行う。

| ドメイン | 書き手 | 契機 |
|---|---|---|
| お知らせ本体・翻訳 | support | 管理 UI からの CRUD 操作（§ 管理 UI） |
| 問い合わせ受付 | support | 外部フォームからの POST |
| 問い合わせステータス・対応メモ | support | Slack アクションボタン → slack-commands → 内部 API |

他サービスとの状態同期は **行わない**（Pub/Sub publish なし）。support は他サービスを直接呼ばない。

## API / 管理 UI / 外部フォームの分離（3 ポート運用）

support は同一バイナリ・同一 Pod で 3 つのポートを listen し、信頼境界で分ける。

| ポート | プロトコル | 想定クライアント | 認証 | Kubernetes Service |
|---|---|---|---|---|
| `:9009` | HTTP JSON | gateway / slack-commands | ClusterIP 内部 | ClusterIP |
| `:9109` | HTTP HTML + HTMX | 運用者ブラウザ | IAP が `X-Goog-Authenticated-User-Email` を付与 | 外部 Ingress + IAP |
| `:9209` | HTTP JSON | 問い合わせフォーム（ブラウザ） | なし（CORS で Origin 制限） | 外部 Ingress |

### 1 ポート + path 振り分けにしない理由

- 外部公開面と内部 API を同一ポートで束ねると、「`/internal/*` を誤って Ingress に露出させた」単一の manifest ミスで内部 API が外部到達可能になる
- IAP は Ingress / LB 単位で設定するため、`/admin/*` だけに IAP 認証を強制することはできない。`/admin/*` を公開ポートと混在させると、誤って ClusterIP 側に露出する構造的リスクが生まれる
- ポートごとに Service / Ingress を分けることで、「管理 UI は IAP 経由のみ」「問い合わせフォームは外部 Ingress のみ」「gateway / slack-commands は ClusterIP 内のみ」という信頼境界を manifest レベルで固定できる
- CORS の適用範囲もポート単位に閉じ込められる（`:9209` のみ Origin 許可、他ポートは CORS 無効）

### CORS

`:9209` の `/api/v1/inquiries` は問い合わせフォームのオリジンからのみ受ける。許可オリジンは env で指定。`:9009` / `:9109` 側は CORS を設定しない。

## IAP による管理 UI 認証の肩代わり

管理 UI は認証ロジックを support コード内に持たない。

```
ブラウザ → GCLB → IAP ─(認証成功時のみ)→ GKE Ingress → support :9109
                  │
                  └─ X-Goog-Authenticated-User-Email ヘッダを付与
```

support 側の `iapMiddleware` はヘッダ存在確認と context への email 注入のみ行う（将来「誰が編集したか」を記録する場合に備えた土台）。許可ユーザーリストの管理は IAP 側に閉じる:

- 運用者追加のたびに support のデプロイが必要になるのを避ける
- IAP の IAM 管理 UI で完結させる方が運用者にとって自然
- ヘッダ偽装の防御は「ポート `:9109` を ClusterIP / 外部 JSON Ingress に露出させない」という manifest 側の契約で担保する（k8s manifest 変更時はこの前提を検証する）

### ENV=local での認証スキップ

ローカル開発では `iapMiddleware` がパススルーしヘッダ無しで通る。この分岐は `ENV` env var のみで制御し、コードフラグで制御しない（「production で誤ってスキップが有効になる」リスクを env 設定に集約）。

## HTMX レンダリング層の構造

管理 UI は `html/template` + `embed.FS` でバイナリに埋め込まれた HTML を動的生成する。FE 成果物は独立に存在しない（news と同一パターン）。

レイヤー配置は他の delivery 経路（REST）と同じく Clean Architecture に従う:

- **handler 層**: 管理 UI の HTTP ルーティング・IAP middleware・テンプレート描画を担当する。REST handler とは別ルータを構築する（3 ポート構成）
- **service 層**: お知らせ CRUD ユースケースは管理 UI handler と将来的な REST handler が共通で呼ぶ。UI 固有のロジックを service に持たせない
- **repository 層**: 管理 UI は同じ repo を読み書きするだけで、admin 固有の SQL を持たない
- **テンプレート / static**: handler 層に同居し `embed.FS` でバイナリに同梱する

### 部分レスポンスの契約

HTMX は `hx-target` / `hx-swap` で DOM の一部を差し替える。新規作成・更新・削除は **ページ全体ではなくフラグメントだけを返す** か、HX-Redirect で遷移させる。フルページを返すハンドラと部分を返すハンドラを混在させると HTMX 側の `hx-target` 指定ミスが発生しやすいため、以下を契約とする:

| ハンドラ種別 | 返すもの |
|---|---|
| `GET /admin/announcements` | フルページ HTML |
| `GET /admin/announcements/new` | フルページ HTML |
| `GET /admin/announcements/:announcementId` | フルページ HTML |
| `POST /admin/announcements` (新規作成) | `200 OK` + `HX-Redirect: /admin/announcements/:id` |
| `POST /admin/announcements/:announcementId` (本体更新) | `200 OK` + `HX-Redirect: /admin/announcements` |
| `POST /admin/announcements/:announcementId/delete` | 差し替え用の `<tr>` フラグメント（削除後の行消去）または `HX-Redirect: /admin/announcements` |
| `POST /admin/announcements/:announcementId/translations/:lang` | 該当言語タブの差し替え用フラグメント |

### XSS 対策

`html/template` はコンテキスト対応のエスケープを行う。お知らせ本文・タイトルは運用者入力だが、将来的に外部インポートを行う可能性もあるため **文字列連結で HTML を組み立てない**（厳守）。

## 問い合わせ受付の副作用オーケストレーション

`SubmitInquiry` は DB 書き込み後に 2 つの外部副作用を発火する（Slack 通知 / SendGrid 受付確認メール）。

### 順序は DB → Slack → SendGrid、fail-fast

この順序は以下の意図で決まる:

1. DB が最優先。DB に行が残らなければ、後続の副作用が成功しても運営・ユーザー双方の情報がどこにも残らない
2. Slack は運営向けの早期通知。運営が問い合わせを把握できる状態にしてからユーザーに受付確認を送る
3. SendGrid はユーザー向けの受付確認。Slack が失敗しているときに「受付完了メール」だけが届くと、ユーザーは成立と認識するが運営は気づかない — 最悪の食い違いパターンになるため、Slack 成功を先に確定させる

失敗は fail-fast で即 return。Slack が失敗した時点で SendGrid を打たない（上記の食い違い防止）。呼び出し元にはエラーを返し、DB 行は残るため、[FEATURE_SPEC §8.3](FEATURE_SPEC.md) の内部 API で運営が救済する。

### トランザクション境界を跨ぐのは 1 点のみ

DB 書き込みは INSERT 1 件で完結する（purchaseToken のような外部識別子が無いため冪等性保証は諦めている）。shop の Transactional Outbox と異なり Pub/Sub は噛まない。Slack / SendGrid の失敗リトライは運営手動 or フォーム再送に委ねる。

将来イベント化が必要になった場合（例: 問い合わせ受付時に account サービスへ通知）は shop 同様の Outbox を導入する。

## Slack / SendGrid クライアントの注入境界

Slack 通知・SendGrid 送信は **adapter 層のクライアント** として注入する。service 層は `port.SlackNotifier` / `port.EmailSender` を介して呼び、具体実装を知らない。

- `ENV=local` では両 port にモック実装を注入し、外部送信を抑止する（Slack / SendGrid の dev token を開発者 PC に配布しなくて済む）
- slack-commands との契約は「support → Slack にメッセージ投稿」「slack-commands → support の内部 API 呼び出し」の 2 方向で、相互に REST / webhook で切り離す。Slack bot token は slack-commands 側が管理し、support は投稿 API のラッパ利用のみ
- SendGrid テンプレートは外部サービス（SendGrid 管理画面）に置かず、**support バイナリ内の `html/template`** で組み立てる。プロバイダ変更時は `port.EmailSender` の adapter を付け替えるだけで済み、テンプレートの再登録・同期が不要

## お知らせ翻訳テーブル分離の意図

お知らせ本体 (`announcements`) と翻訳 (`announcement_translations`) を別テーブルに分けているのは:

- 翻訳の後出し追加（`ja` だけ先に公開し、後日 `en` を追加）を本体更新なしで行いたい
- 特定言語の翻訳だけ差し替えても、他言語の配信・監査ログに影響させない
- `published_at` / `expires_at` などの公開制御を言語と独立に持たせる（言語単位で公開期間を分けるのは UX 上害が多いと判断）

指定 `lang` の翻訳が存在しない記事はフォールバックせず、一覧から除外・詳細で 404 を返す（CLAUDE.md「デフォルト値へのフォールバックを行わない」に従う）。翻訳の未整備を運営に対して「サイレントに成功しない」ことで可視化し、運用者責務で補完させる。

新規作成時は本体と ja 翻訳を **同一トランザクション** で INSERT する（[FEATURE_SPEC §6.4](FEATURE_SPEC.md)）。翻訳 0 件のお知らせが作られない状態を DB レベルで保証し、公開契約を構造的に守る。詳細なスキーマは [DATA_DESIGN.md](DATA_DESIGN.md) を参照。

## 下書きは `published_at = NULL` で表現する

お知らせの状態（下書き / 予約公開 / 公開中 / 期限切れ）は `published_at` と `expires_at` の 2 列だけで表現し、`status` 列を持たない:

- 専用の状態遷移 API (`publish` / `unpublish`) を持たず、`published_at` の UPDATE 1 つで公開・非公開が切り替わる
- 予約公開と即時公開の区別が DDL レベルで不要（どちらも `published_at` に timestamp を入れるだけ）
- 公開 API (`:9009`) は `published_at IS NOT NULL AND published_at <= now` でフィルタするため、下書きが誤って露出する経路が構造的に存在しない

`status` 列と状態遷移 API を持つ news とは方針が異なる。news は外部インジェスト由来で「校閲」という明示的な承認フローが必要だが、support のお知らせは運営が手で作るものなので、フロー自体を簡素化できる。

## スパム対策を現時点で入れていない理由

問い合わせ受付エンドポイントは gateway を経由せず外部直通で、認証も無い。CAPTCHA / レート制限 / honeypot フィールド等のスパム対策は優先度低として現時点では実装しない。

運用負荷の観測から導入要否を判断する:

- Slack 通知があるため、運営が流入を即時に観測できる
- 大量スパムが観測されたタイミングで `:9209` の前段に Cloud Armor / reCAPTCHA 等を入れる前提とする
- SendGrid 側のドメイン reputation が不正メールで損なわれた場合も同様に導入契機とする

「やらない」のは能動的な意思決定として残す。CLAUDE.md「デフォルト値へのフォールバックを行わない」とは別領域の判断。

## 運用

### 環境変数 / Secret Manager

環境変数の一覧と必須条件は `internal/config/config.go` が SSoT（起動時に検証、欠ければ即 fail）。

- Slack bot token / channel ID は Secret Manager から起動時取得
- SendGrid API key / from アドレス・差出人名は Secret Manager から取得（テンプレート ID は SendGrid 側に置かないので不要）
- IAP 関連の許可ユーザーリストは IAP 側に閉じる（support の env には持たない）
- ローカル開発では `make run` が env を自動注入する

### 外部依存とヘルスチェック

`/health` は DB 接続のみチェックする。Slack / SendGrid の外部連携は健康チェック対象に含めない（外部サービスのメンテで support が unhealthy 扱いになると、API そのものは生きているのに Ingress から切り離されるため）。Slack / SendGrid の不調は副作用失敗時の ERROR ログと運用監視（Cloud Logging アラート）で検知する。

### 管理 UI の可用性

管理 UI (`:9109`) は運用者の業務時間中のみ使われる内部ツール。24/7 SLO は設けない。公開 API (`:9009` / `:9209`) に影響しなければダウンタイムを許容。
