# PinguCoin の開発用コマンド。リポジトリのルートで実行する。
#   make help   ... コマンド一覧

# 統合テスト用の使い捨てPostgres(make test-db)
TEST_DB_NAME := pg-test
TEST_DB_PORT := 55432
TEST_DB_URL  := postgres://postgres:postgres@localhost:$(TEST_DB_PORT)/pingucoin_test?sslmode=disable

.PHONY: help build test test-db proto tidy up down

help: ## コマンド一覧を表示する
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  make %-10s %s\n", $$1, $$2}'

build: ## 全サービスをビルドする
	go build ./...

test: ## 単体テストを実行する(DBを使う統合テストはスキップされる)
	go test ./...

test-db: ## 使い捨てのPostgresを起動して、統合テストを含む全テストを実行する
	-docker rm -f $(TEST_DB_NAME) >/dev/null 2>&1
	docker run -d --name $(TEST_DB_NAME) -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=pingucoin_test -p $(TEST_DB_PORT):5432 postgres:16-alpine
	@until docker exec $(TEST_DB_NAME) pg_isready -U postgres -d pingucoin_test >/dev/null 2>&1; do sleep 1; done
	TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... ; status=$$?; docker rm -f $(TEST_DB_NAME) >/dev/null; exit $$status

proto: ## .proto を検査して、Goのコードを生成し直す(services/<service>/proto/)
	buf lint
	buf generate services/orcan-api/proto --template services/orcan-api/proto/buf.gen.yaml
	buf generate services/payment-api/proto --template services/payment-api/proto/buf.gen.yaml

tidy: ## go.mod / go.sum を整える
	go mod tidy

up: ## minikube にAPIを起動する(イメージのビルドから)
	bash script/start.sh

down: ## minikube のAPIを停止する
	bash script/stop.sh
