# Local development tooling

This folder contains tooling for local development.

It makes use of Docker Compose to build and run a local api and a local database.

## Commands

For a validator stack against a local hardhat network:
- `make up`
- `make down`

For a validator stack against different real EVM chain enviroments run:
- `make {network-name}-up` 
- `make {network-name}-down`

Note that every enviroment has two files:
- `.env_{network-name}` which allows to configure enviroments to fill secrets for the validator, plus expand variables present in the config file.
- `config-{network-name}.json` is the configuration file for the validator.


To connect to the database run:
- `make psql`
