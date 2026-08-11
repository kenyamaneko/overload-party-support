# overload-party-support

お知らせ配信を行う内部マイクロサービス。起動するのは gateway 向け内部 REST のポート 9009 のみ。

詳細は [API仕様書](data/openapi.yaml) / [データ設計書](docs/DATA_DESIGN.md) を参照。設計判断 (Why) は [common の ADR](https://github.com/kenyamaneko/overload-party-common/tree/main/docs/adr) に記録する。

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
