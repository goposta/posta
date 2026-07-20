.PHONY: build build-ui build-all run run-worker dev dev-worker dev-deps test vet fmt lint clean

BINARY := posta
BUILD_DIR := bin
UI_DIR := web
# Where `go build` picks the dashboard up for embedding (see internal/web).
EMBED_UI_DIR := internal/web/dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X github.com/goposta/posta/internal/config.Version=$(VERSION) -X github.com/goposta/posta/internal/config.CommitID=$(COMMIT)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/posta

build-ui: ## Build the dashboard and stage it for embedding (internal/web/dist)
	cd $(UI_DIR) && npm install && npm run build
	# Stage the build output where //go:embed reads it, keeping the committed
	# .gitkeep so `go build` still works on a clean tree.
	rm -rf $(EMBED_UI_DIR)
	cp -r $(UI_DIR)/dist $(EMBED_UI_DIR)
	touch $(EMBED_UI_DIR)/.gitkeep

build-all: build-ui build ## Build the dashboard, then a self-contained binary

run: build
	./$(BUILD_DIR)/$(BINARY) server

run-worker: build
	./$(BUILD_DIR)/$(BINARY) worker

run-ui:
	cd $(UI_DIR) && npm run dev

dev: build
	./$(BUILD_DIR)/$(BINARY) server

dev-worker: build
	./$(BUILD_DIR)/$(BINARY) worker

dev-ui:
	cd $(UI_DIR) && npm run dev

dev-deps:
	docker run -d --name posta-postgres \
		-e POSTGRES_USER=posta \
		-e POSTGRES_PASSWORD=posta \
		-e POSTGRES_DB=posta \
		-p 5432:5432 \
		postgres:17-alpine
	docker run -d --name posta-redis \
		-p 6379:6379 \
		redis:7-alpine

dev-deps-stop:
	docker stop posta-postgres posta-redis || true
	docker rm posta-postgres posta-redis || true

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(EMBED_UI_DIR)
	mkdir -p $(EMBED_UI_DIR) && touch $(EMBED_UI_DIR)/.gitkeep
	rm -rf $(UI_DIR)/dist

tidy:
	go mod tidy

lint:
	golangci-lint run