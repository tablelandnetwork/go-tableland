include .bingo/Variables.mk

HEAD_SHORT ?= $(shell git rev-parse --short HEAD)
PLATFORM ?= $(shell uname -m)

BIN_VERSION?="git"

HTTP_PORT ?= 8080

GO_BINDATA=go run github.com/go-bindata/go-bindata/v3/go-bindata@v3.1.3
SQLC=go run github.com/kyleconroy/sqlc/cmd/sqlc@v1.15.0
GOVVV=go run github.com/ahmetb/govvv@v0.3.0 

GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(BIN_VERSION) -pkg $(shell go list ./buildinfo))

# Code generation
ethereum: ethereum-testcontroller ethereum-testerc721 ethereum-testerc721a
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.20 --abi ./pkg/tables/impl/ethereum/abi.json --pkg ethereum --type Contract --out pkg/tables/impl/ethereum/contract.go --bin pkg/tables/impl/ethereum/bytecode.bin
.PHONY: ethereum

ethereum-testcontroller:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.20 --abi ./pkg/tables/impl/ethereum/test/controller/abi.json --pkg controller --type Contract --out pkg/tables/impl/ethereum/test/controller/controller.go --bin pkg/tables/impl/ethereum/test/controller/bytecode.bin
.PHONY: ethereum-testcontroller

ethereum-testerc721:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.20 --abi ./pkg/tables/impl/ethereum/test/erc721Enumerable/abi.json --pkg erc721Enumerable --type Contract --out pkg/tables/impl/ethereum/test/erc721Enumerable/erc721Enumerable.go --bin pkg/tables/impl/ethereum/test/erc721Enumerable/bytecode.bin
.PHONY: ethereum-testerc721

ethereum-testerc721a:
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.10.20 --abi ./pkg/tables/impl/ethereum/test/erc721aQueryable/abi.json --pkg erc721aQueryable --type Contract --out pkg/tables/impl/ethereum/test/erc721aQueryable/erc721aQueryable.go --bin pkg/tables/impl/ethereum/test/erc721aQueryable/bytecode.bin
.PHONY: ethereum-testerc721a

system-sql-assets:
	cd pkg/sqlstore/impl/system && $(GO_BINDATA) -pkg migrations -prefix migrations/ -o migrations/migrations.go -ignore=migrations.go migrations && $(SQLC) generate; cd -;
.PHONY: system-sql-assets

mocks: clean-mocks
	go run github.com/vektra/mockery/v2@v2.14.0 --name='\b(?:SQLRunner|Tableland)\b' --recursive --with-expecter
.PHONY: mocks

clean-mocks:
	rm -rf mocks
.PHONY: clean-mocks

EVM_EVENTS_ORIGIN:="docker/deployed/testnet/api/backup_database.db"
EVM_EVENTS_TARGET:="pkg/eventprocessor/impl/testdata/evm_history.db"
generate-history-db:
	rm -f ${EVM_EVENTS_TARGET}
	sqlite3 ${EVM_EVENTS_ORIGIN} 'ATTACH DATABASE ${EVM_EVENTS_TARGET} as target' 'CREATE TABLE target.system_evm_events as select * from system_evm_events'
	zstd -f ${EVM_EVENTS_TARGET}
	rm ${EVM_EVENTS_TARGET}

print-govvv:
	echo "${GOVVV_FLAGS}"
.PHONY: .print-govvv

# Build 
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

# Test
test: 
	go test ./... -short -race
.PHONY: test

test-replayhistory:
	go test ./pkg/eventprocessor/impl -run=TestReplayProductionHistory -race

# Lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1 run
.PHONY: lint

# OpenAPI
SPEC_URL=https://raw.githubusercontent.com/tablelandnetwork/docs/main/specs/validator/tableland-openapi-spec.yaml
APIV1=${PWD}/internal/router/controllers/apiv1
gen-api-v1:
	mkdir -p ${APIV1}
	curl -s ${SPEC_URL} > ${APIV1}/tableland-openapi-spec.yaml
	docker run -w /gen -e GEN_DIR=/gen -v ${APIV1}:/gen swaggerapi/swagger-codegen-cli-v3:3.0.36 \
	   generate --lang go-server -o /gen -i tableland-openapi-spec.yaml --additional-properties=packageName=apiv1 
	sudo chown -R ${USER} ${APIV1} 
	cd ${APIV1} && \
	   mv go/* . && \
	   rm -rf go main.go Dockerfile README.md api .swagger-codegen .swagger-codegen-ignore *.yaml
	sed -i 's/\*OneOfTableAttributesValue/interface{}/' internal/router/controllers/apiv1/model_table_attributes.go
.PHONY: gen-api-v1