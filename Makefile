include .bingo/Variables.mk

HEAD_SHORT ?= $(shell git rev-parse --short HEAD)
PLATFORM ?= $(shell uname -m)

BIN_BUILD_FLAGS?=CGO_ENABLED=0
BIN_VERSION?="git"
GOVVV_FLAGS=$(shell $(GOVVV) -flags -version $(BIN_VERSION) -pkg $(shell go list ./buildinfo))

HTTP_PORT ?= 8080
GCP_PROJECT=textile-310716

build-api: $(GOVVV)
	$(BIN_BUILD_FLAGS) go build -ldflags="${GOVVV_FLAGS}" ./cmd/api
.PHONY: build-api

image:
	docker build --platform linux/amd64 -t tableland/api:sha-$(HEAD_SHORT) -t tableland/api:latest .
.PHONY: image

run:
	docker run -d --name api -p ${HTTP_PORT}:8080 tableland/api
.PHONY: run

stop:
	docker stop api
	docker rm api
.PHONY: stop

publish:
	docker tag tableland/api:sha-$(HEAD_SHORT) us-west1-docker.pkg.dev/${GCP_PROJECT}/textile/tableland/api:sha-$(HEAD_SHORT)
	docker push us-west1-docker.pkg.dev/${GCP_PROJECT}/textile/tableland/api:sha-$(HEAD_SHORT)
.PHONY: publish