include .bingo/Variables.mk

HEAD_SHORT ?= $(shell git rev-parse --short HEAD)
PLATFORM ?= $(shell uname -m)

BIN_BUILD_FLAGS?=CGO_ENABLED=0
BIN_VERSION?="git"

HTTP_PORT ?= 8080
GCP_PROJECT=textile-310716

GO_BINDATA=go run github.com/go-bindata/go-bindata/v3/go-bindata@v3.1.3
SQLC=go run github.com/kyleconroy/sqlc/cmd/sqlc@v1.11.0
GOVVV=go run github.com/ahmetb/govvv@v0.3.0 

GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(BIN_VERSION) -pkg $(shell go list ./buildinfo))

# Code generation

ethereum:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/ethereum/abi.json --pkg ethereum --type Contract --out pkg/tableregistry/impl/ethereum/contract.go --bin pkg/tableregistry/impl/ethereum/registry.bin
.PHONY: ethereum

system-sql-assets:
	cd pkg/sqlstore/impl/system && $(GO_BINDATA) -pkg migrations -prefix migrations/ -o migrations/migrations.go -ignore=migrations.go migrations && $(SQLC) generate; cd -;
.PHONY: system-sql-assets

# Local development with docker-compose

up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-dev.yml | COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f - up --build
.PHONY: up

down:
	docker-compose -f docker-compose-dev.yml down
.PHONY: down

psql:
	docker-compose -f docker-compose-dev.yml exec api psql postgresql://dev_user:dev_password@database/dev_database
.PHONY: psql

# Building and publishing image to GCP

build-api:
	$(BIN_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/api
.PHONY: build-api

image:
	docker build --platform linux/amd64 -t tableland/api:sha-$(HEAD_SHORT) -t tableland/api:latest -f ./cmd/api/Dockerfile .
.PHONY: image

publish:
	docker tag tableland/api:sha-$(HEAD_SHORT) us-west1-docker.pkg.dev/${GCP_PROJECT}/textile/tableland/api:sha-$(HEAD_SHORT)
	docker push us-west1-docker.pkg.dev/${GCP_PROJECT}/textile/tableland/api:sha-$(HEAD_SHORT)
.PHONY: publish

# Test

test:
	go test -v ./...
.PHONY: test

# Lint

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.43.0 run
.PHONYY: lint