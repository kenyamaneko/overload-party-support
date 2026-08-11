# セットアップ

## ローカル開発

`make run` はアプリ本体とインフラ (Postgres) を compose 内で起動する。インフラはホストへ公開せず
内部ネットワークのサービス名 DNS で参照するため、他リポのローカルスタックやホスト上の他アプリと
ポートが衝突しない。ホストへ出るのは support の API ポート (内部 9009) のみ。

```bash
make run      # アプリ + インフラを compose で起動（ソースをバインドマウント）
make down     # 停止して volume を削除
make test     # Testcontainers でテスト実行（Docker 必須）
```

アプリはコンテナ内で `go run` する。ソースを編集して `docker compose restart support` すれば、
イメージを作り直さずに反映される。プライベートモジュールはホストのモジュールキャッシュを読み取り専用でマウント
して解決するため、`make run` は先にホスト側で `go mod download` を実行する。
