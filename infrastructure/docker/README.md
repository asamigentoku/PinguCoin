# infrastructure/docker

ローカル開発用のインフラ(PostgreSQL / Redis / Kafka / Azure Blob Storageエミュレーター)を
Docker Composeで起動する。

Azuriteは[Azure Blob Storage](https://learn.microsoft.com/azure/storage/blobs/)のローカルエミュレーターで、
Queue/Tableは使わずBlobのみ起動している(`azurite-blob`コマンド)。

## サービスごとの設定

各サービスの環境変数は別ファイルに切り出さず、`docker-compose.local.yml`の
各サービスの`environment`に直接書く。ローカル開発用の値のみ(パスワード等も
秘密情報ではない固定値)なので、`.env`ファイルに分ける必要が無いため。
Postgresの認証情報やKafkaのブローカー設定を変えたい場合は、この
`docker-compose.local.yml`を直接編集する。

## 起動・停止

```bash
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml down
```

`orcan-api` のデフォルト設定(`server/orcan-api/.env.example`)はこのPostgres
(`localhost:5432`, DB `orcan`)にそのまま繋がるようになっている。

## ポート

| サービス | ポート |
| --- | --- |
| PostgreSQL | 5432 |
| Redis | 6379 |
| Kafka | 9092 |
| Azurite(Blob) | 10000 |

## Kafka(KRaftモード)

ZooKeeperなしの単一ブローカー構成(`apache/kafka`公式イメージ)。
`KAFKA_LISTENERS`のうち`PLAINTEXT`/`CONTROLLER`はコンテナのホスト名(`kafka`)で
バインドする必要があり、`0.0.0.0`にすると起動時に
`advertised.listeners cannot use the nonroutable meta-address 0.0.0.0` で失敗するので注意
(外部公開用の`PLAINTEXT_HOST`だけ`0.0.0.0`でよい)。

## Azurite接続情報

ローカルからの接続は標準の開発用アカウントを使う(`AZURITE_ACCOUNTS`を指定しない場合のデフォルト)。

```
AccountName: devstoreaccount1
AccountKey:  Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==
BlobEndpoint: http://localhost:10000/devstoreaccount1
```

## 将来的な「develop」環境について

実際のクラウドリソース(Azure Database for PostgreSQL / Azure Cache for Redis /
Event Hubs Kafka互換エンドポイント / 実Azure Blob Storage)に接続する
develop環境が必要になったら、`docker-compose.develop.yml`ではなく
各サービスの`.env.<環境名>.example`を追加して接続文字列を切り替える形にする
(コンテナ構成自体はlocalと変わらないため、compose定義を複製する意味は薄い)。
