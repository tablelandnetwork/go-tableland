FROM golang:1.19.6 as builder

RUN apt-get install make git

RUN mkdir /app
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download -x
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
  BIN_BUILD_FLAGS="GOOS=linux" make build-api

FROM debian

COPY --from=builder /app/api /app/api
WORKDIR /app
ENTRYPOINT ["./api"]
