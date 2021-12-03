include .bingo/Variables.mk

HEAD_SHORT ?= $(shell git rev-parse --short HEAD)
PLATFORM ?= $(shell uname -m)

BIN_BUILD_FLAGS?=CGO_ENABLED=0
BIN_VERSION?="git"
GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(BIN_VERSION) -pkg $(shell go list ./buildinfo))

HTTP_PORT ?= 8080
GCP_PROJECT=textile-310716

# Code generation

contracts:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/erc1155/abi.json --pkg erc1155 --type Contract --out pkg/tableregistry/impl/erc1155/contract.go
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.13 --abi ./pkg/tableregistry/impl/erc721/abi.json --pkg erc721 --type Contract --out pkg/tableregistry/impl/erc721/contract.go
.PHONY: contract

# Local development with docker-compose

up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-dev.yml | COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f - up --build
.PHONY: up

down:
	docker-compose -f docker-compose-dev.yml down
.PHONY: down

# Building and publishing image to GCP

build-api: $(GOVVV)
	$(BIN_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/api
.PHONY: build-api

image:
	docker build --platform linux/amd64 -t tableland/api:sha-$(HEAD_SHORT) -t tableland/api:latest -f ./cmd/api/Dockerfile .
.PHONY: image

publish:
	docker tag tableland/api:sha-$(HEAD_SHORT) us-west1-docker.pkg.dev/${GCP_PROJECT}/textile/tableland/api:sha-$(HEAD_SHORT)
	docker push us-west1-docker.pkg.dev/${GCP_PROJECT}/textile/tableland/api:sha-$(HEAD_SHORT)
.PHONY: publish