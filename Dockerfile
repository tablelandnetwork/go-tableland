FROM golang:1.17-buster as builder

RUN mkdir /app 
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN BIN_BUILD_FLAGS="CGO_ENABLED=0 GOOS=linux" make build-api

FROM alpine

#RUN apk --no-cache add postgresql-client
#COPY /bin/with-postgres.sh /app/

COPY --from=builder /app/api /app/api
WORKDIR /app
ENTRYPOINT ["./api"]