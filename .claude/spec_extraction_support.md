# overload-party-support 仕様抽出 (テスト全削除後の再構築用)

common#171 の横断是正 (support 分、Issue #24) の一環。個別是正でなく、実装をなぞる既存テストを一旦全削除し (PR: https://github.com/kenyamaneko/overload-party-support/pull/63)、公開経路から検証する形で仕様ベースに書き直す。本ドキュメントはテスト生成の前工程である仕様抽出の結果であり、テストコードそのものは含まない。

## 前提

### ブラックボックス境界の考え方

対象コードの公開経路 (exported な関数・型・HTTP ハンドラ) を検証対象とする。ただし `internal/port` の各インタフェース (`AnnouncementQuerier` など) はクリーンアーキテクチャにおける DI 境界として意図的に用意されたものであり、この境界をテストダブルに差し替えて内側のロジックを検証することは許容する。実 DB・実 Cloud SQL など、DI 境界の外側にある本物の外部サービスに直接アクセスする実装は、後述のとおり結合テストとして扱う。

`cmd/server` (`package main`) は他パッケージから import できず、公開経路を用意できない。この 1 箇所に限り、非公開関数をパッケージ内部から直接呼び出して検証する (理由は該当節に記載)。

### CI 実行の実態

- ビルドタグ (`//go:build integration` 等) はリポ内のどのファイルにも存在しない
- `.github/workflows/ci.yaml` は `has-integration-tests: false` を指定しており、タグ付き結合テスト専用ジョブ (`test-integration`) は動かない
- 実 DB を使うテスト (後述の「結合テスト」指定箇所) は、タグで隔離されず `test-unit` ジョブの `go test ./...` に混在する形で実行される。Docker が使えることが前提 (`postgrestest` が testcontainers で Postgres を起動する)
- 実 DB を使うテストを含むパッケージ (`internal/repository/postgres`、`internal/handler/rest`) は、それぞれ独自に `TestMain` を用意し `postgrestest.RunMain` でコンテナを起動する必要がある (既存テスト全削除により、いずれの `TestMain` も現時点では存在しない)

### 命名との対応

各仕様項目の Given/Then は `rules/lang/go.md` の対応 (テスト対象要素 = 直下の `t.Run`、各ケース = ケースごとの `t.Run`) に従って変換される想定で書いている。

---

## お知らせドメインモデル (internal/domain)

対象: `IsSupportedLang` (`announcement_helpers.go`)、`TypeInfo`/`TypeMaintenance`/`TypeEvent`/`TypeUpdate` (`announcement_enum.go`) と `data/openapi.yaml` の契約整合。単体テスト。

### 対応言語の判定 (IsSupportedLang)

lang の対応可否そのものの境界値はここでのみ確認する。他レイヤ (usecase) では、対応可否の判定結果としてどちらの sentinel error になるかだけを確認し、この境界値を再度列挙しない。

- lang が `ja` のとき、対応言語と判定される
- lang が `en` のとき、対応言語と判定される
- lang が対応言語一覧に無い値 (例: `fr`) のとき、対応言語でないと判定される
- lang が空文字のとき、対応言語でないと判定される
- lang の大文字小文字が対応言語と異なる値 (例: `JA`) のとき、対応言語でないと判定される (大文字小文字を区別する)

### お知らせ種別と openapi.yaml の契約整合

DB・外部 API へのアクセスは無いが `data/openapi.yaml` を実ファイルとして読むため、通常の単体テストとは別に「ファイルを読む検証」であることを明記する。domain 側が種別の正とする値集合の SSoT であり、openapi.yaml 側は独立に同じ値集合を定義する契約になっている。期待値をどちらの実装 (定数・enum 定義) からも取り込まず、テスト側のリテラルとして両方に書くことで、どちらが変化しても drift を検知できるようにする。

- お知らせ種別の定数の集合が、リテラル `info` / `maintenance` / `event` / `update` の集合と一致する
- `data/openapi.yaml` の `components.schemas.AnnouncementType.enum` の値集合が、同じリテラル `info` / `maintenance` / `event` / `update` の集合と一致する

---

## 起動設定 (internal/config)

対象: `FromEnv` (`config.go`)。単体テスト (プロセス環境変数のみに依存し、外部サービスへのアクセスは無い)。

- ENV が `local` のとき、設定が構築され、動作環境の設定値は `local` になる
- ENV が `staging` のとき、設定が構築され、動作環境の設定値は `staging` になる
- ENV が `production` のとき、設定が構築され、動作環境の設定値は `production` になる
- ENV が未設定のとき、エラーになる
- ENV が `local` / `staging` / `production` のいずれでもない値のとき、エラーになる
- INTERNAL_PORT が未設定のとき、エラーになる
- INTERNAL_PORT が整数でない値のとき、エラーになる
- DATABASE_CONN が未設定のとき、エラーになる
- DATABASE_IAM_AUTH_ENABLED が未設定のとき、エラーになる
- DATABASE_IAM_AUTH_ENABLED が `true` / `false` のいずれでもない値のとき、エラーになる
- DATABASE_IAM_AUTH_ENABLED が `true` かつ CLOUDSQL_CONNECTION_NAME が未設定のとき、エラーになる
- DATABASE_IAM_AUTH_ENABLED が `true` かつ CLOUDSQL_CONNECTION_NAME が設定されているとき、設定が構築され、CLOUDSQL_CONNECTION_NAMEの値が反映される
- DATABASE_IAM_AUTH_ENABLED が `false` のとき、構築された設定のCLOUDSQL_CONNECTION_NAMEは空になる
- INTERNAL_PORT と DATABASE_CONN に設定した値が、構築された設定の対応するフィールドにそのまま反映される

---

## 公開お知らせユースケース (internal/usecase/announcement)

対象: `Usecase.List`、`Usecase.GetDetail` (`usecase.go`)。単体テスト。お知らせ取得ポート (`port.AnnouncementQuerier`) をテストダブルに差し替えて検証する (DI 境界であり許容する)。

lang の対応可否の境界値そのものは「お知らせドメインモデル」の「対応言語の判定 (IsSupportedLang)」の仕様に従う。ここでは判定結果としてどちらの sentinel error になるかだけを確認する。

### List

- lang が空文字のとき、`announcement.ErrLangRequired` を返す
- lang が対応外の値のとき、`announcement.ErrUnsupportedLang` を返す
- lang が対応言語のとき、指定した lang と現在時刻をお知らせ取得ポートへ渡す
- お知らせ取得ポートが結果を返したとき、その結果をそのまま呼び出し元に返す
- お知らせ取得ポートが想定外のエラーを返したとき、そのエラーを握りつぶさず、その原因のエラーだと判別できる形で呼び出し元に伝播する

### GetDetail

- lang が空文字のとき、`announcement.ErrLangRequired` を返す
- lang が対応外の値のとき、`announcement.ErrUnsupportedLang` を返す
- lang が対応言語のとき、指定した announcementID と lang をお知らせ取得ポートへ渡す
- お知らせ取得ポートが結果を返したとき、その結果をそのまま呼び出し元に返す
- お知らせ取得ポートが `port.ErrNotFound` を返したとき、`announcement.ErrNotFound` に変換して返す
- お知らせ取得ポートが `port.ErrNotFound` 以外の想定外のエラーを返したとき、そのエラーを握りつぶさず、その原因のエラーだと判別できる形で呼び出し元に伝播する

---

## お知らせリポジトリ (internal/repository/postgres)

対象: `AnnouncementRepository.ListPublished`、`AnnouncementRepository.GetPublishedDetail` (`announcement_repo.go`)。

**全項目が結合テスト。** 実 Postgres (`internal/repository/postgres/postgrestest` が提供する testcontainers ベースのコンテナ) に対して実行する。パッケージ単位で `TestMain` を用意し `postgrestest.RunMain` でコンテナを起動、各ケースの前に `Truncate` してテーブルを空にしてから行を用意する。

### ListPublished

- 公開日時が現在時刻以前で、かつ期限日時が未設定または現在時刻より後で、指定langの翻訳が存在する行が結果に含まれる
- 公開日時が未設定 (下書き) の行は結果に含まれない
- 公開日時が現在時刻より後 (公開前) の行は結果に含まれない
- 期限日時が現在時刻以前 (期限日時と現在時刻が同時刻の場合を含む) の行は結果に含まれない
- 期限日時が未設定の行は、期限切れとして除外されない
- 公開日時が現在時刻と同時刻の行は結果に含まれる
- 指定langの翻訳が存在しない行は結果に含まれない (他langの翻訳しかない行は対象外になる)
- 公開条件を満たす行が複数あるとき、公開日時の降順で並ぶ
- 公開日時が同じ行同士は、announcement_idの降順で並ぶ
- 該当する行が無いとき、要素数0の結果を返す

### GetPublishedDetail

- 指定announcementIdの行に指定langの翻訳が存在するとき、公開日時が未設定 (下書き) でも取得できる
- 指定announcementIdの行に指定langの翻訳が存在するとき、公開日時が現在時刻より後 (公開前) でも取得できる
- 指定announcementIdの行に指定langの翻訳が存在するとき、公開日時が現在時刻以前 (公開中) でも取得できる
- 指定announcementIdの行が存在しないとき、`port.ErrNotFound` を返す
- 指定announcementIdの行は存在するが指定langの翻訳が存在しないとき、`port.ErrNotFound` を返す

**製品判断として確認が必要な事項**: 上記の3項目 (下書き/公開前/公開中いずれでも取得できる) は `internal/port/announcement.go` の docs コメントと `data/openapi.yaml` の `getAnnouncement` summary に明記された意図的な仕様である。実装上は公開日時・期限日時のどちらによる絞り込みも行わず、announcement_id と lang の一致のみで行を返すため、期限切れのお知らせも同様に取得できる。ただし announcement_id を推測されると、下書き・公開前・期限切れのお知らせも gateway 経由で閲覧可能になる。テストで固定する前に、この挙動を維持するかどうかの確認を推奨する。

---

## 内部APIルータ (internal/router)

対象: `NewInternal` (`internal.go`)。

- `GET /health` を呼び出すと、ステータス200と本文 `{"status":"ok"}` を返す

ルーティング自体 (`/api/v1/support/announcements` 系のパス) は、次節の handler 仕様のテストが `router.NewInternal` の返す実物のルータを使って検証することでカバーする。テスト側でルートを再定義しない (再定義すると `internal.go` 側のパス誤りを検出できなくなるため)。

---

## 公開お知らせAPI (internal/handler/rest)

対象: `AnnouncementHandler.List` (`GET /api/v1/support/announcements`)、`AnnouncementHandler.GetDetail` (`GET /api/v1/support/announcements/:announcementId`)。テストは `router.NewInternal` が返す実物のルータにHTTPリクエストを送って検証する。

ステータスコードへの変換規則 (`announcement.ErrNotFound` → 404、`announcement.ErrLangRequired` / `announcement.ErrUnsupportedLang` → 400、それ以外の想定外エラー → 500) のうち、どの状況でどの sentinel error になるかは「公開お知らせユースケース」の仕様に従う。ここでは対応するステータスコードになることを確認する。ただし同じステータスコードでも原因が異なる分岐 (400の2種類、GetDetailの404の2種類) は、応答本文の `error` の値まで確認し、原因を取り違えても検知できるようにする。

### List (GET /api/v1/support/announcements)

単体テスト。お知らせ取得ポートをテストダブルに差し替える。

- lang クエリパラメータが無いとき、ステータス400と本文 `{"error":"announcement: lang is required"}` を返す
- lang クエリパラメータが対応外の値のとき、ステータス400と本文 `{"error":"announcement: unsupported lang"}` を返す
- お知らせ取得ポートが想定外のエラーを返すとき、ステータス500を返す
- お知らせ取得ポートが結果としてnil (該当なし) を返すとき、応答本文の `announcements` はnullでなく空配列になる
- 取得結果が複数件あるとき、応答本文の `announcements` の各要素にannouncement_id/type/title/published_atが取得結果のとおり反映される

### GetDetail (GET /api/v1/support/announcements/:announcementId)

単体テスト。お知らせ取得ポートをテストダブルに差し替える。

- announcementIdが数値としてパースできない値のとき、ステータス404と本文 `{"error":"announcement not found"}` を返す
- lang クエリパラメータが無いとき、ステータス400と本文 `{"error":"announcement: lang is required"}` を返す
- lang クエリパラメータが対応外の値のとき、ステータス400と本文 `{"error":"announcement: unsupported lang"}` を返す
- お知らせ取得ポートが `port.ErrNotFound` を返すとき、ステータス404と本文 `{"error":"announcement: not found"}` を返す
- お知らせ取得ポートが想定外のエラーを返すとき、ステータス500を返す
- お知らせが取得できたとき、応答本文にannouncement_id/type/title/body/published_atが取得結果のとおり反映される
- 取得結果のpublished_atが未設定のとき、応答本文のpublished_atはnullになる

### エンドツーエンドの配線確認

**結合テスト。** 実Postgres (`postgrestest`) 上に構築した本物の `AnnouncementRepository` と実物の `router.NewInternal` を使い、テストダブルを挟まずHTTPリクエストからDBまで一気通貫で検証する。router → handler → usecase → repository → DB の配線全体が正しく繋がっていることの確認が目的であり、個々の分岐は上記の単体テストでカバー済みのため、代表的な成功・失敗を1件ずつ確認すれば足りる。

- 実DBに公開条件を満たすお知らせを登録した状態でそのannouncementIdを指定すると、ステータス200でその内容が返る
- 実DBに存在しないannouncementIdを指定すると、ステータス404が返る

---

## 応答詰め替え (internal/presenter)

仕様項目は無い。対象: `ToAnnouncementSummary`、`ToAnnouncementSummaries`、`ToAnnouncementDetail` (`announcement.go`)。フィールドの詰め替えに加え、`ToAnnouncementSummaries` はnilを要素数0のスライスに変換する保証を持つ。この保証を含め、応答本文へのフィールド反映は「公開お知らせAPI」の仕様 (List/GetDetailの各項目) で確認するため、本レイヤ単独の仕様項目は用意しない。

---

## プロセス起動 (cmd/server)

対象: `setupLogger`、`serve` (`main.go`)。両関数とも非公開だが、`main` パッケージは他パッケージから import できず他に検証経路が無いため、パッケージ内部から直接呼び出して検証する。

### setupLogger

- 動作環境が `local` / `staging` / `production` のいずれかのとき、エラーにならない
- 動作環境がそれ以外の値のとき、エラーになる

`newCloudLoggingHandler` (ログ属性名を Cloud Logging 向けにリネームする処理) は、書き込み先が `os.Stdout` に固定されておりテストから出力を捕捉する手段が無いため、仕様項目を用意しない。

### serve

- 指定したリスナーで待ち受けを開始し、設定したハンドラがリクエストに応答する
- 停止が指示されると、graceful shutdownを行いエラー無く終了する

Shutdown失敗時にエラーを返す分岐は、外部からShutdownを意図的に失敗させる手段が無いため仕様項目を用意しない。

**対象外**: `run()` は設定読み込み・DB接続プール構築・ハンドラ組み立てなど配線が中心で、個々の要素は他レイヤ (config/repository/usecase/handler) の仕様でカバーする。`run()` 自体を対象にした仕様項目は用意しない。

---

## packages/api-support/apisupportclient (公開クライアントライブラリ)

対象: `Client.GetHealth`、`Client.ListAnnouncements`、`Client.GetAnnouncement`、`New`、`WithHTTPClient`、`WithRequestEditorFn` (`client.go`)。単体テスト。HTTP通信を実際に行うテスト用サーバ (`apisupportserverfake` または直接構築した `httptest.Server`) を相手に検証する (生成クライアント内部を直接差し替えない)。

`New` は現状の実装ではbaseURLの妥当性検証を行わないため (どんな文字列を渡しても構築自体は成功する)、baseURLの検証に関する仕様項目は無い。

### ステータスコードからのエラー変換 (List/Get/Health 共通)

`ListAnnouncements`/`GetAnnouncement`/`GetHealth` はいずれも、200番台以外の応答を同じ規則でエラーに変換する。以下は `ListAnnouncements` を例に全区分を確認する。他の2メソッドでは、この規則に従うことと呼び出し元のメソッド名がエラーメッセージに含まれることだけを確認すれば足りる。

- 応答が400のとき、`apisupportclient.ErrBadRequest` を返す
- 応答が401のとき、`apisupportclient.ErrUnauthorized` を返す
- 応答が404のとき、`apisupportclient.ErrNotFound` を返す
- 応答が500以上のとき、`apisupportclient.ErrInternalServer` を返す
- 応答が400・401・404・500以上のいずれの区分にも該当しない (例: 300) とき、`apisupportclient.ErrBadRequest`・`apisupportclient.ErrUnauthorized`・`apisupportclient.ErrNotFound`・`apisupportclient.ErrInternalServer` のいずれでもない、呼び出し元の操作名とステータスコードを含むエラーを返す

### GetHealth

- 応答が200のとき、応答本文をそのまま返す
- 応答が500のとき、`apisupportclient.ErrInternalServer` を返し、エラーメッセージに操作名 `GetHealth` が含まれる

### ListAnnouncements

- 呼び出し時、指定したlangをクエリパラメータとして送信する
- 応答が200のとき、応答本文をそのまま返す
- 応答が404のとき、`apisupportclient.ErrNotFound` を返し、エラーメッセージに操作名 `ListAnnouncements` が含まれる

### GetAnnouncement

- 呼び出し時、指定したannouncementIDとlangをリクエストとして送信する
- 応答が200のとき、応答本文をそのまま返す
- 応答が400のとき、`apisupportclient.ErrBadRequest` を返し、エラーメッセージに操作名 `GetAnnouncement` が含まれる

### リクエストオプション

- HTTPクライアントを差し替えて指定したとき、そのクライアントを介してリクエストが送信される
- リクエストを編集する処理を設定すると、送信する全てのリクエストにその処理が適用される

---

## packages/api-support/apisupportserverfake (テスト用フェイクサーバ)

対象: `Server` (`server.go`)。単体テスト。実際にHTTPリクエストを送って検証する。他リポ (gateway等) がsupportのフェイクとして使う公開ライブラリのため、契約どおりに動くことを直接確認する。

### お知らせ一覧エンドポイント (GET /api/v1/support/announcements)

- お知らせ一覧を返す処理を設定していないとき、ステータス200で `announcements` が空配列の本文を返す
- お知らせ一覧を返す処理を設定しているとき、その処理にlangクエリパラメータの値が渡る
- お知らせ一覧を返す処理を設定しているとき、その処理の戻り値のステータスと本文がそのまま応答になる
- お知らせ一覧を返す処理の戻り値の本文がnilのとき、応答本文は空になる

### お知らせ詳細エンドポイント (GET /api/v1/support/announcements/{announcementId})

- announcementIdが数値としてパースできない値のとき、ステータス404を返す
- announcementIdが数値としてパースできない値のとき、お知らせ詳細を返す処理を呼ばない
- お知らせ詳細を返す処理を設定していないとき、ステータス404を返す
- お知らせ詳細を返す処理を設定しているとき、その処理にint64へパースしたannouncementIdの値とlangクエリパラメータの値が渡る
- お知らせ詳細を返す処理を設定しているとき、その処理の戻り値のステータスと本文がそのまま応答になる
- お知らせ詳細を返す処理の戻り値の本文がnilのとき、応答本文は空になる

---

## 対象外の箇所と理由

- **`cmd/server/db.go`** (`newDatabasePool` / `newDatabasePoolWithIAMAuth` / `closeDialer`): 実Cloud SQLへのIAM認証接続が絡み、テスト環境に実Cloud SQLが無いと分岐の一方を検証できない。既存テストも元々無かった。着手前にテスト方針をユーザーに確認する対象として明記する
- **`internal/router/internal.go` の `newRequestLogger`**: レスポンスステータスに応じたログレベルの出し分けを行う。ログの出力先を差し替えれば出力内容自体は観測できるが、その方式でテストを書くかどうかの方針が定まっていない。着手前にテスト方針をユーザーに確認する対象として明記する
- **`packages/api-support/openapi_gen.go`**: `data/openapi.yaml` から生成されたコードで、`make generate-types` の対象。手書きロジックを持たないため個別の仕様項目を起票しない。同ファイルの `AnnouncementType.Valid()` はどこからも呼ばれておらず未使用
- **`internal/port`** (`AnnouncementQuerier` インタフェース定義、`ErrNotFound` センチネル、`MockAnnouncementRepo`): インタフェース定義とテストダブルのみで独自ロジックを持たない。上位レイヤ (usecase/repository) の仕様内で使う
- **`db/schema.sql` の `support.inquiries` テーブル**: 現行コードにこのテーブルへアクセスする実装が無い (問い合わせ機能はPR#53で削除済みの残存スキーマ)。対応する仕様項目は無い

## 製品判断として確認が必要な事項 (まとめ)

- 「お知らせリポジトリ」の `GetPublishedDetail` が、announcement_idとlangが一致する限り公開日時・期限日時のいずれにも関わらず (下書き・公開前・期限切れを含め) 取得できる仕様を、テストで固定してよいか
- 「対象外の箇所と理由」の `cmd/server/db.go` について、実Cloud SQLが無い環境でテストをどう扱うか
- 「対象外の箇所と理由」の `internal/router/internal.go` の `newRequestLogger` について、ログ出力先を差し替える形でテストを書くか
