# overload-party-support

お知らせ配信と問い合わせ受付を行う内部マイクロサービス。内部 REST（gateway 向け）はポート 9009、運用者向け管理 UI は IAP 背後の 9109、外部問い合わせフォーム向け REST は 9209 で起動する。

詳細は [機能仕様書](docs/FEATURE_SPEC.md) / [サービス設計書](docs/ARCHITECTURE.md) / [API仕様書](data/openapi.yaml) / [データ設計書](docs/DATA_DESIGN.md) を参照。

[テスト観点カタログ](https://kenyamaneko.github.io/overload-party-support/): テスト名から生成した、テスト済みの観点の一覧。

## アーキテクチャ概要

```
Gateway
  └─ Support (:9009 internal REST)
       ├─ PostgreSQL (support スキーマ)
       ├─ Slack    (受付通知を運営チャンネルへ投稿)
       └─ SendGrid (問い合わせ者宛の受付確認メール)

運用者ブラウザ
  └─ IAP (Google OAuth)
       └─ Support (:9109 admin UI)  ← お知らせ CRUD / 問い合わせ管理

問い合わせフォーム (gateway を経由しない)
  └─ Support (:9209 external REST)
       └─ POST /api/v1/inquiries
```

他サービスへの状態同期は行わない（Pub/Sub publish 無し）。Slack と SendGrid は support からの一方向呼び出し。

## ローカル開発

`make run` はアプリ本体とインフラ (Postgres) を compose 内で起動する。インフラはホストへ publish せず
内部ネットワークのサービス名 DNS で参照するため、他リポのローカルスタックやホスト上の他アプリと
ポートが衝突しない。ホストへ出るのは support の API ポート (internal 9009 / admin 9109 / external 9209) のみ。

```bash
make run      # アプリ + インフラを compose で起動（ソース bind-mount）
make down     # 停止して volume を削除
make test     # Testcontainers でテスト実行（Docker 必須）
```

アプリはコンテナ内で `go run` する。ソースを編集して `docker compose restart support` すれば、
イメージを作り直さずに反映される。private module は host の module cache を読み取り専用でマウント
して解決するため、`make run` は先に host 側で `go mod download` を実行する。

ローカル起動時 (`ENV=local`) は IAP middleware がパススルーし、`http://localhost:9109/admin/` に直接アクセスできる。Slack / SendGrid クライアントもモック実装にフォールバックするため、dev token を開発者 PC に配布する必要はない。

## 公開パッケージ

[packages/api-support/](packages/api-support/) に REST 契約型を公開している。[data/openapi.yaml](data/openapi.yaml) を編集後に以下で再生成する。

```bash
make generate-types   # oapi-codegen をローカルにインストールしておくこと
```
