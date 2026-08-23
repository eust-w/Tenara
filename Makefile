TENARA_BASE_DOMAIN ?= 127.0.0.1.nip.io

GO_MODULES := control-plane controllers analyzer builder verifier providers
BIN_DIR := bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
GOFUMPT := $(BIN_DIR)/gofumpt
GOFUMPT_ABS := $(abspath $(GOFUMPT))
export PATH := $(abspath $(BIN_DIR)):$(PATH)

.PHONY: lint lint-go lint-ts test test-go tools test-mcp-conformance \
	dev-up dev-down dev-reset kind-up kind-down generate \
	e2e-smoke build-images helm-install migrate-up migrate-down

## tools: install project-local toolchain (golangci-lint v2 into ./bin)
GOFUMPT := $(BIN_DIR)/gofumpt

tools: $(GOLANGCI_LINT) $(GOFUMPT)

$(GOLANGCI_LINT):
	GOBIN=$(abspath $(BIN_DIR)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1

$(GOFUMPT):
	GOBIN=$(abspath $(BIN_DIR)) go install mvdan.cc/gofumpt@latest

node_modules:
	pnpm install --no-frozen-lockfile

## lint: full static analysis gate (Go + TS)
lint: lint-go lint-ts lint-contract

lint-go: tools
	@set -e; for m in $(GO_MODULES); do \
		echo "== gofumpt $$m"; (cd "$$m" && test -z "$$($(GOFUMPT_ABS) -l .)"); \
		echo "== golangci-lint $$m"; (cd "$$m" && golangci-lint run ./...); \
	done

lint-ts: node_modules
	pnpm exec eslint .
	pnpm exec prettier --check .

test-mcp-conformance:
	cd e2e/mcp && node conformance.mjs

lint-contract:
	pnpm exec spectral lint api/openapi.yaml

## test-secrets: secrets package acceptance (plan todo 19)
test-secrets:
	cd control-plane && go test ./internal/secrets/... -v

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
	docker compose -f deploy/docker-compose.yml up -d --wait
dev-down:
	docker compose -f deploy/docker-compose.yml down
dev-reset:
	docker compose -f deploy/docker-compose.yml down -v
	docker compose -f deploy/docker-compose.yml up -d --wait
kind-up:
	kind get clusters 2>/dev/null | grep -qx tenara || kind create cluster --config deploy/kind/kind-config.yaml
	kubectl apply -f deploy/kind/calico.yaml
	kubectl -n kube-system rollout status ds/calico-node --timeout=300s
kind-down:
	kind delete cluster --name tenara || true
gateway-up:
	bash scripts/dev-certs.sh
	kubectl apply -f deploy/kind/gateway.yaml
	kubectl apply -f deploy/kind/demo-httpecho.yaml
	@sleep 5
	curl -s --noproxy '*' --resolve demo.$(TENARA_BASE_DOMAIN):443:127.0.0.1 \
		--cacert "$$(mkcert -CAROOT)/rootCA.pem" https://demo.$(TENARA_BASE_DOMAIN)/ | grep -q hello
gateway-down:
	kubectl delete -f deploy/kind/gateway.yaml --ignore-not-found
	kubectl delete -f deploy/kind/demo-httpecho.yaml --ignore-not-found
generate:
	$(BIN_DIR)/oapi-codegen -config control-plane/internal/gen/cfg.yaml api/openapi.yaml
	pnpm exec openapi-typescript api/openapi.yaml -o sdk/ts/src/generated/schema.ts
e2e-smoke:
	cd e2e && pnpm install --no-frozen-lockfile >/dev/null
	kubectl -n tenara-system port-forward svc/tenara-control-plane 18080:80 >/tmp/tenara-pf.log 2>&1 & \
	PF_PID=$$!; sleep 3; \
	cd e2e && pnpm exec playwright test -g smoke; status=$$?; \
	kill $$PF_PID 2>/dev/null; exit $$status
build-images:
	bash scripts/build-images.sh
helm-install: 
	helm upgrade --install tenara-platform deploy/helm/tenara-platform \
		--namespace tenara-system --create-namespace \
		-f deploy/helm/tenara-platform/values-dev.yaml \
		-f deploy/helm/build-digests.yaml --wait --timeout 300s
migrate-up:
	GOBIN=$(abspath $(BIN_DIR)) go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3
	@if [ -n "$$(ls control-plane/internal/store/migrations/*.sql 2>/dev/null)" ]; then \
		$(BIN_DIR)/migrate -path control-plane/internal/store/migrations -database "$(TENARA_DATABASE_URL)" up; \
	else echo "no migrations yet"; fi
migrate-new:
	@test -n "$(name)" || (echo "usage: make migrate-new name=add_x"; exit 1)
	$(BIN_DIR)/migrate create -ext sql -dir control-plane/internal/store/migrations -seq "$(name)"
