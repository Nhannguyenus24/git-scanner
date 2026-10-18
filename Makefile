SHELL := /bin/bash

MODULE   := github.com/mservice/gitscanner
SERVICES := api discovery-worker scan-worker enricher notifier analytics
REGISTRY ?= localhost:5050
TAG      ?= dev
GOBIN    := $(shell go env GOPATH)/bin

BUF_VERSION           := v1.58.0
SQLC_VERSION          := v1.30.0
GOLANGCI_LINT_VERSION := v2.6.2

export PATH := $(GOBIN):$(PATH)

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

.PHONY: test
test: ## Run unit tests with the race detector
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Run integration tests against real backends
	go test -race -count=1 -tags=integration ./...

.PHONY: check
check: vet lint test ## Everything CI runs

## ---------------------------------------------------------------- build

.PHONY: build
build: ## Compile all services to ./bin
	@mkdir -p bin
	@for s in $(SERVICES); do \
		echo "building $$s"; \
		CGO_ENABLED=0 go build -trimpath -o bin/$$s ./cmd/$$s || exit 1; \
	done

.PHONY: images
images: ## Build container images for the local registry
	@for s in $(SERVICES); do \
		target=runtime; \
		if [ "$$s" = "scan-worker" ]; then target=runtime-git; fi; \
		echo "building image $$s (target $$target)"; \
		docker build -f build/Dockerfile \
			--build-arg SERVICE=$$s --target $$target \
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
