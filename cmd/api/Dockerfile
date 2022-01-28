FROM golang:1.17.6-alpine as builder

RUN apk --no-cache add make build-base

RUN mkdir /app 
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download -x
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
  BIN_BUILD_FLAGS="GOOS=linux" make build-api

FROM alpine

RUN apk --no-cache add postgresql-client
COPY /local/with-postgres.sh /app/

COPY --from=builder /app/api /app/api
WORKDIR /app
ENTRYPOINT ["./api"]
