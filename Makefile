MAIN_SRC := main.go
BINARY_NAME := simple-kvs-registry

export PATH := $(shell go env GOPATH)/bin:$(PATH)

.PHONY: all
all: swag build

## run: Swagドキュメントを生成してからアプリケーションを起動します
.PHONY: run
run: swag
	@echo "🚀 Starting application..."
	go run $(MAIN_SRC)

## build: Swagドキュメントを生成してからバイナリをビルドします
.PHONY: build
build: swag
	@echo "🏗️  Building binary..."
	go build -o $(BINARY_NAME) $(MAIN_SRC)

## swag: Swaggo のドキュメントを強制解析フラグ付きで自動生成します
.PHONY: swag
swag: deps-swag
	@echo "📝 Generating Swagger docs..."
	swag init --parseDependency --parseInternal

## deps: Goモジュールの依存関係を最新の状態に同期・クリーンアップします
.PHONY: deps
deps:
	@echo "📥 Tidying up Go modules..."
	go mod tidy
	go mod download

## deps-swag: swag コマンドが存在しない場合のみ自動でインストールします
.PHONY: deps-swag
deps-swag:
	@@command -v swag >/dev/null 2>&1 || ( \
		echo "🔧 'swag' command not found. Installing latest swaggo/swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest \
	)

## clean: 生成されたバイナリや古いログファイルを削除します
.PHONY: clean
clean:
	@echo "🧹 Cleaning up..."
	rm -f $(BINARY_NAME)
	@echo "✨ Clean complete."