SHELL := /bin/bash

MODULE   := github.com/mservice/gitscanner
# Every directory under cmd/ is a service. Adding one there is the only step
# needed for build, images and push to pick it up.
SERVICES := $(notdir $(wildcard cmd/*))
# Services that shell out to git and need the runtime-git image (plan.md §13.3).
GIT_SERVICES := scan-worker
REGISTRY ?= localhost:5050
TAG      ?= dev
GOBIN    := $(shell go env GOPATH)/bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null)
LDFLAGS := -s -w \
	-X $(MODULE)/libs/buildinfo.version=$(VERSION) \
	-X $(MODULE)/libs/buildinfo.commit=$(COMMIT)

BUF_VERSION           := v1.58.0
SQLC_VERSION          := v1.30.0
GOLANGCI_LINT_VERSION := v2.6.2

export PATH := $(GOBIN):$(PATH)

.PHONY: services
services: ## List the services found under cmd/
	@echo $(SERVICES)

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------- toolchain

.PHONY: bootstrap
bootstrap: ## Install Go-based dev tools (buf, sqlc, golangci-lint)
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo
	@echo "k3d, helm and tilt are not Go programs; install them with:"
	@echo "  brew install k3d helm tilt-dev/tap/tilt"

## ---------------------------------------------------------------- codegen

.PHONY: proto
proto: ## Lint, check wire compatibility and regenerate protobuf
	cd libs/proto && buf lint
	cd libs/proto && buf breaking --against '../../.git#branch=main,subdir=libs/proto' || \
		echo "note: no main branch to compare against yet"
	cd libs/proto && buf generate

.PHONY: sqlc
sqlc: ## Regenerate typed queries from SQL
	@test -f sqlc.yaml && sqlc generate || echo "no sqlc.yaml yet (phase 1)"

.PHONY: generate
generate: proto sqlc ## Run all code generation

## ---------------------------------------------------------------- quality

.PHONY: fmt
fmt: ## Format Go source
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint, including the §3.2 import rules
	golangci-lint run ./...

# Tests live under test/, not beside the code (test/README.md). Because they are
# in other packages, coverage must be attributed with -coverpkg.
COVERPKG := ./internal/...,./libs/...

.PHONY: test
test: ## Run unit tests with the race detector
	go test -race -count=1 ./test/unit/...

.PHONY: test-integration
test-integration: ## Run integration tests against real backends (needs POSTGRES_URL, ...)
	go test -race -count=1 -tags=integration ./test/integration/...

.PHONY: cover
cover: ## Unit test coverage of internal/ and libs/, report in coverage.html
	go test -count=1 -coverpkg=$(COVERPKG) -coverprofile=coverage.out ./test/unit/...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html

.PHONY: chart-lint
chart-lint: ## Lint the Helm chart and render it for dev and prod
	helm lint deploy/chart -f deploy/values/dev.yaml
	helm template gitscanner deploy/chart -f deploy/values/dev.yaml >/dev/null
	helm template gitscanner deploy/chart --set env=prod >/dev/null

.PHONY: check
check: vet lint test ## Everything CI runs

## ---------------------------------------------------------------- build

.PHONY: build
build: ## Compile all services to ./bin
	@mkdir -p bin
	@for s in $(SERVICES); do \
		echo "building $$s"; \
		CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$$s ./cmd/$$s || exit 1; \
	done

.PHONY: images
images: ## Build container images for the local registry
	@for s in $(SERVICES); do \
		target=runtime; \
		case " $(GIT_SERVICES) " in *" $$s "*) target=runtime-git;; esac; \
		echo "building image $$s (target $$target)"; \
		docker build -f build/Dockerfile \
			--build-arg SERVICE=$$s --target $$target \
			--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
			-t $(REGISTRY)/$$s:$(TAG) . || exit 1; \
	done

.PHONY: push
push: ## Push images to the local registry
	@for s in $(SERVICES); do docker push $(REGISTRY)/$$s:$(TAG) || exit 1; done

## ---------------------------------------------------------------- local k8s

CLUSTER     := gitscanner
INFRA_NET   ?= bridge
REGISTRY_K3D := gitscanner-registry

.PHONY: cluster
cluster: ## Create the local k3d cluster with its registry (plan.md §14.1)
	k3d cluster create $(CLUSTER) \
		--agents 2 \
		--registry-create $(REGISTRY_K3D):0.0.0.0:5050 \
		--network $(INFRA_NET)
	helm repo add kedacore https://kedacore.github.io/charts
	helm repo update
	helm upgrade --install keda kedacore/keda \
		--namespace keda --create-namespace --wait
	kubectl create namespace gitscanner --dry-run=client -o yaml | kubectl apply -f -

.PHONY: cluster-down
cluster-down: ## Delete the local cluster
	k3d cluster delete $(CLUSTER)

.PHONY: deploy
deploy: ## Install or upgrade the chart in the local cluster
	helm upgrade --install gitscanner deploy/chart \
		--namespace gitscanner \
		--values deploy/values/dev.yaml \
		--set image.registry=$(REGISTRY) \
		--set image.tag=$(TAG) \
		--wait

.PHONY: up
up: ## Start the Tilt inner loop
	tilt up

.PHONY: down
down: ## Stop Tilt and remove its resources
	tilt down

## ---------------------------------------------------------------- frontend

.PHONY: frontend-install
frontend-install: ## Install frontend npm dependencies
	cd frontend && npm install

.PHONY: frontend-dev
frontend-dev: ## Start Vite dev server on :3000 (proxies /v1 to :8080)
	cd frontend && npm run dev

.PHONY: frontend-build
frontend-build: ## Build frontend for production into frontend/dist
	cd frontend && npm run build

## ---------------------------------------------------------------- local run (no k8s)

INFRA_POSTGRES  ?= postgresql://postgres:postgres123@localhost:5432/app_db
INFRA_RABBIT    ?= amqp://admin:admin123@localhost:5672/
RUN_LOG_DIR     ?= /tmp/gitscanner-logs

.PHONY: run-local
run-local: build ## Build and start all six services locally (no k8s). Logs → RUN_LOG_DIR.
	@mkdir -p $(RUN_LOG_DIR)
	@export POSTGRES_URL='$(INFRA_POSTGRES)' RABBITMQ_URL='$(INFRA_RABBIT)' \
	        PROBE_SECRETS='$(PROBE_SECRETS)' \
	        LOG_LEVEL=debug ENV=dev; \
	port=8080; \
	for svc in api discovery-worker scan-worker enricher notifier analytics; do \
	    echo "starting $$svc on :$$port"; \
	    HTTP_ADDR=":$$port" ./bin/$$svc >$(RUN_LOG_DIR)/$$svc.log 2>&1 & \
	    echo $$! >$(RUN_LOG_DIR)/$$svc.pid; \
	    [ "$$svc" = "api" ] && sleep 2; \
	    port=$$((port+1)); \
	done; \
	sleep 1; \
	echo ""; \
	echo "PIDs:"; \
	for f in $(RUN_LOG_DIR)/*.pid; do \
	    s=$$(basename $$f .pid); p=$$(cat $$f); \
	    kill -0 $$p 2>/dev/null && echo "  $$s pid=$$p RUNNING" || echo "  $$s pid=$$p DEAD"; \
	done; \
	echo ""; \
	echo "Health:"; \
	for port in 8080 8081 8082 8083 8084 8085; do \
	    r=$$(curl -sf http://localhost:$$port/readyz 2>/dev/null && echo OK || echo FAIL); \
	    echo "  :$$port /readyz -> $$r"; \
	done

.PHONY: stop-local
stop-local: ## Stop all services started by run-local
	@for f in $(RUN_LOG_DIR)/*.pid; do \
	    [ -f "$$f" ] || continue; \
	    s=$$(basename $$f .pid); p=$$(cat $$f); \
	    kill "$$p" 2>/dev/null && echo "stopped $$s ($$p)" || echo "$$s already gone"; \
	    rm -f "$$f"; \
	done
