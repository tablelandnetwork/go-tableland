include .bingo/Variables.mk

HEAD_SHORT ?= $(shell git rev-parse --short HEAD)
PLATFORM ?= $(shell uname -m)

BIN_VERSION?="git"

HTTP_PORT ?= 8080
GCP_PROJECT=textile-310716

GO_BINDATA=go run github.com/go-bindata/go-bindata/v3/go-bindata@v3.1.3
SQLC=go run github.com/kyleconroy/sqlc/cmd/sqlc@v1.13.0
GOVVV=go run github.com/ahmetb/govvv@v0.3.0 

GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(BIN_VERSION) -pkg $(shell go list ./buildinfo))

# Code generation

ethereum: ethereum-testcontroller ethereum-testerc721 ethereum-testerc721a
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.17 --abi ./pkg/tables/impl/ethereum/abi.json --pkg ethereum --type Contract --out pkg/tables/impl/ethereum/contract.go --bin pkg/tables/impl/ethereum/bytecode.bin
.PHONY: ethereum

ethereum-testcontroller:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.17 --abi ./pkg/tables/impl/ethereum/test/controller/abi.json --pkg controller --type Contract --out pkg/tables/impl/ethereum/test/controller/controller.go --bin pkg/tables/impl/ethereum/test/controller/bytecode.bin
.PHONY: ethereum-testcontroller

ethereum-testerc721:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.17 --abi ./pkg/tables/impl/ethereum/test/erc721Enumerable/abi.json --pkg erc721Enumerable --type Contract --out pkg/tables/impl/ethereum/test/erc721Enumerable/erc721Enumerable.go --bin pkg/tables/impl/ethereum/test/erc721Enumerable/bytecode.bin
.PHONY: ethereum-testerc721

ethereum-testerc721a:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.17 --abi ./pkg/tables/impl/ethereum/test/erc721aQueryable/abi.json --pkg erc721aQueryable --type Contract --out pkg/tables/impl/ethereum/test/erc721aQueryable/erc721aQueryable.go --bin pkg/tables/impl/ethereum/test/erc721aQueryable/bytecode.bin
.PHONY: ethereum-testerc721a


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

build-api-debug:
	go build -ldflags="${GOVVV_FLAGS}" -gcflags="all=-N -l" ./cmd/api
.PHONY: build-api-debug

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
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.46.2 run
.PHONYY: lint
