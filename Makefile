include .bingo/Variables.mk

HEAD_SHORT ?= $(shell git rev-parse --short HEAD)
PLATFORM ?= $(shell uname -m)

BIN_VERSION?="git"

HTTP_PORT ?= 8080
GCP_PROJECT=textile-310716

GO_BINDATA=go run github.com/go-bindata/go-bindata/v3/go-bindata@v3.1.3
SQLC=go run github.com/kyleconroy/sqlc/cmd/sqlc@v1.11.0
GOVVV=go run github.com/ahmetb/govvv@v0.3.0 

GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(BIN_VERSION) -pkg $(shell go list ./buildinfo))

# Code generation

ethereum:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/ethereum/abi.json --pkg ethereum --type Contract --out pkg/tableregistry/impl/ethereum/contract.go --bin pkg/tableregistry/impl/ethereum/bytecode.bin
.PHONY: ethereum

ethereum-controller:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/ethereum//controller/abi.json --pkg controller --type Contract --out pkg/tableregistry/impl/ethereum/controller/badges_controller.go --bin pkg/tableregistry/impl/ethereum/controller/bytecode.bin
.PHONY: ethereum

ethereum-badges:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/ethereum//badges/abi.json --pkg badges --type Contract --out pkg/tableregistry/impl/ethereum/badges/badges.go --bin pkg/tableregistry/impl/ethereum/badges/bytecode.bin
.PHONY: ethereum

ethereum-rigs:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/ethereum//rigs/abi.json --pkg rigs --type Contract --out pkg/tableregistry/impl/ethereum/rigs/rigs.go --bin pkg/tableregistry/impl/ethereum/rigs/bytecode.bin
.PHONY: ethereum


system-sql-assets:
	cd pkg/sqlstore/impl/system && $(GO_BINDATA) -pkg migrations -prefix migrations/ -o migrations/migrations.go -ignore=migrations.go migrations && $(SQLC) generate; cd -;
.PHONY: system-sql-assets

# Building and publishing image to GCP

build-api:
	go build -ldflags="${GOVVV_FLAGS}" ./cmd/api
.PHONY: build-api

build-healthbot:
	go build -ldflags="${GOVVV_FLAGS}" ./cmd/healthbot
.PHONY: build-healthbot

build-api-dev:
	go build -ldflags="${GOVVV_FLAGS}" -gcflags="all=-N -l" ./cmd/api
.PHONY: build-api-dev

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
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.45.2 run
.PHONYY: lint
