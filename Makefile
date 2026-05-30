.PHONY: run-api run-worker migrate proto-gen web-dev web-build lint test docker-build

# Load .env if present
-include .env
export

run-api:
	go run ./cmd/dashmap serve

run-worker:
	go run ./cmd/dashmap worker

migrate:
	go run ./cmd/dashmap migrate

proto-gen:
	buf generate

web-dev:
	cd internal/web && npm run dev

web-build:
	cd internal/web && npm run build

GO_PKGS := $(shell go list ./cmd/... ./internal/... | grep -v node_modules)

lint:
	go vet $(GO_PKGS)
	golangci-lint run ./cmd/... ./internal/...
	cd internal/web && npm run lint

test:
	go test $(GO_PKGS)

test-integration:
	go test -tags integration $(GO_PKGS)

docker-build:
	docker buildx build --platform linux/amd64 -t dashmap:dev .
