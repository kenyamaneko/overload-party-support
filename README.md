# overload-party-support

お知らせ配信を行う内部マイクロサービス。起動するのは gateway 向け内部 REST のポート 9009 のみ。

お知らせの管理 UI と問い合わせ受付フォームは提供していない。お知らせの登録・更新は運用者が手作業で行い、問い合わせは外部フォームで受ける。将来は管理機能と問い合わせを別サービスとして用意する予定で、本リポの管理 UI と問い合わせ受付のコードとドキュメントはその設計の下敷きとして残している。

詳細は [機能仕様書](docs/FEATURE_SPEC.md) / [サービス設計書](docs/ARCHITECTURE.md) / [API仕様書](data/openapi.yaml) / [データ設計書](docs/DATA_DESIGN.md) を参照。

[テスト観点カタログ](https://kenyamaneko.github.io/overload-party-support/): テスト名から生成した、テスト済みの観点の一覧。

## アーキテクチャ概要

```
Gateway
  └─ Support (:9009 internal REST)
       └─ PostgreSQL (support スキーマ)
```

他サービスへの状態同期は行わない（Pub/Sub publish 無し）。

## ローカル開発

`make run` はアプリ本体とインフラ (Postgres) を compose 内で起動する。インフラはホストへ publish せず
内部ネットワークのサービス名 DNS で参照するため、他リポのローカルスタックやホスト上の他アプリと
ポートが衝突しない。ホストへ出るのは support の API ポート (internal 9009) のみ。

```bash
make run      # アプリ + インフラを compose で起動（ソース bind-mount）
make down     # 停止して volume を削除
make test     # Testcontainers でテスト実行（Docker 必須）
```

アプリはコンテナ内で `go run` する。ソースを編集して `docker compose restart support` すれば、
イメージを作り直さずに反映される。private module は host の module cache を読み取り専用でマウント
して解決するため、`make run` は先に host 側で `go mod download` を実行する。

## 公開パッケージ

[packages/api-support/](packages/api-support/) に REST 契約型を公開している。[data/openapi.yaml](data/openapi.yaml) を編集後に以下で再生成する。

```bash
make generate-types   # oapi-codegen をローカルにインストールしておくこと
```
