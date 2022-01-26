# Local development tooling

This folder contains tooling for local development.

It makes use of Docker Compose to build and run a local api and a local database.

## Commands

- `make up`: to run a local Tableland API connected to a local database
- `make down`: stops the local api and local database containers
- `make psql`: connects to the database via api
