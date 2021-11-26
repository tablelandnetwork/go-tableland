# Go Tableland

This project is part of a POC of the Tableland project.

![Tableland](https://user-images.githubusercontent.com/1233473/143463929-e5e0ee72-6dc6-4dda-a444-ea15268f227b.png)

It implements the validator as a JSON-RPC server responsible for updating a Postgres database.

## API Spec

[Postman Collection](https://www.postman.com/aviation-participant-86342471/workspace/my-workspace/collection/18493329-068ef574-afde-4057-926c-ebee6628315c)

## Current state of the project (and design decisions)

- Only the `CreateTable` feature is implemented in the The JSON-RPC server. All other features are mocked with fixed responses
- The JSON-RPC is implemented using Ethereum's [implementation](https://pkg.go.dev/github.com/ethereum/go-ethereum/rpc) of the [2.0 spec](https://www.jsonrpc.org/specification)
- The JSON-RPC server is an HTTP server (Just for clarification. It could also be just a TCP server.)
- The server is currently deployed as a `docker` container inside a [Compute Engine VM](https://console.cloud.google.com/compute/instances?project=textile-310716&authuser=1)
- Configs can be passed with flags, config.json file or env variables (it uses the [uConfig](https://github.com/omeid/uconfig) package)
- There is a Postgres database running inside the same [Compute Engine VM](https://console.cloud.google.com/compute/instances?project=textile-310716&authuser=1) as the container
- For local development, there is a `docker-compose` file. Just execute `make up` to have the validator up and running.

## How to publish a new version

This project uses `docker` and Google's [Artifact Registry](https://console.cloud.google.com/artifacts?authuser=1&project=textile-310716) for managing container images.

```bash
make image    # builds the image
make publish  # publishes to Artifact Registry
```

Make sure you have `gcloud` installed and configured.
If you get an error while trying to publish, try to run `gcloud auth configure-docker us-west1-docker.pkg.dev`

## How to deploy

```bash
docker run -d --name api -p 80:8080 --add-host=database:172.17.0.1 -e DB_HOST=database -e DB_PASS=[[PASSWORD]] -e DB_USER=validator -e DB_NAME=tableland -e DB_PORT=5432 [[IMAGE]]
```

## Next steps

- Add Infura (Ethereum) integration
- Implement the main use cases (CreateTable, UpdateTable, RunSQL)
