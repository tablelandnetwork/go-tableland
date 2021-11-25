# Go Tableland

This project is part of a POC of the Tableland project.

It implements a JSON-RPC server responsible for updating a Postgres database. It can be seen as a fake validator of the Tableland blockchain.

## API Spec

[Postman Collection](https://www.postman.com/aviation-participant-86342471/workspace/my-workspace/collection/18493329-068ef574-afde-4057-926c-ebee6628315c)

## Current state of the project (and design decisions)

- The JSON-RPC server is just a mock with fixed responses
- The JSON-RPC is implemented using Ethereum's [implementation](https://pkg.go.dev/github.com/ethereum/go-ethereum/rpc) of the [2.0 spec](https://www.jsonrpc.org/specification)
- The JSON-RPC server is an HTTP server (Just for clarification. It could also be just a TCP server.)
- The server is currently deployed as a `docker` container inside a [Compute Engine VM](https://console.cloud.google.com/compute/instances?project=textile-310716&authuser=1)
- Configs can be passed with flags, config.json file or env variables (it uses the [uConfig](https://github.com/omeid/uconfig) package)

## How to publish a new version

This project uses `docker` and Google's [Artifact Registry](https://console.cloud.google.com/artifacts?authuser=1&project=textile-310716) for managing container images.

```bash
make image    # builds the image
make publish  # publishes to Artifact Registry
```

Make sure you have `gcloud` installed and configured.
If you get an error while trying to publish, try to run `gcloud auth configure-docker us-west1-docker.pkg.dev`

## Next steps

- Setup a Postgres database
- Add Postgres integration
- Add Infura (Ethereum) integration
- Implement the main use cases (CreateTable, UpdateTable, RunSQL)
