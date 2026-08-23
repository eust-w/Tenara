GO_MODULES := control-plane controllers analyzer builder verifier providers
BIN_DIR := bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
export PATH := $(abspath $(BIN_DIR)):$(PATH)

.PHONY: lint lint-go lint-ts test test-go tools \
	dev-up dev-down dev-reset kind-up kind-down generate \
	e2e-smoke build-images helm-install migrate-up migrate-down

## tools: install project-local toolchain (golangci-lint v2 into ./bin)
tools: $(GOLANGCI_LINT)

$(GOLANGCI_LINT): go.work
	GOBIN=$(abspath $(BIN_DIR)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

node_modules:
	pnpm install --no-frozen-lockfile

## lint: full static analysis gate (Go + TS)
lint: lint-go lint-ts

lint-go: tools
	@set -e; for m in $(GO_MODULES); do \
		echo "== golangci-lint $$m"; (cd "$$m" && golangci-lint run ./...); \
	done

lint-ts: node_modules
	pnpm exec eslint .
	pnpm exec prettier --check .

## test: unit/test gate across all modules
test: test-go
	pnpm -r --if-present run test

test-go: 
	@set -e; for m in $(GO_MODULES); do \
		echo "== go test $$m"; \
		(cd "$$m" && go build ./... && go vet ./... && go test -race -shuffle=on -count=1 ./...); \
	done

# ---- placeholders wired by later plan todos ----

dev-up:
	@echo "dev-up: not wired yet (plan todo 2)"; exit 0
dev-down:
	@echo "dev-down: not wired yet (plan todo 2)"; exit 0
dev-reset:
	@echo "dev-reset: not wired yet (plan todo 2)"; exit 0
kind-up:
	@echo "kind-up: not wired yet (plan todo 3)"; exit 0
kind-down:
	@echo "kind-down: not wired yet (plan todo 3)"; exit 0
generate:
	@echo "generate: not wired yet (plan todo 5)"; exit 0
e2e-smoke:
	@echo "e2e-smoke: not wired yet (plan todo 7)"; exit 0
build-images:
	@echo "build-images: not wired yet (plan todo 6)"; exit 0
helm-install:
	@echo "helm-install: not wired yet (plan todo 6)"; exit 0
migrate-up:
	@echo "migrate-up: not wired yet (plan todo 9)"; exit 0
migrate-down:
	@echo "migrate-down: not wired yet (plan todo 9)"; exit 0
