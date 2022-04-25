# Local development tooling

This folder contains tooling for local development.

It makes use of Docker Compose to build and run a local api and a local database.

## Commands

For a validator stack against a local hardhat network:
- `make up`
- `make down`

For a validator stack against the staging enviroment running in the L2 testnet:
- `make staging-up`
- `make staging-down`
Note that you need an `.env` file with your Alchemy API KEY in the L2 testnet:
```
ALCHEMY_API_KEY=XXXXXXXXX
```

To connect to the database run:
- `make psql`
