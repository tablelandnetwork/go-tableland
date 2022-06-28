# Local development tooling

This folder contains tooling for local development.

It makes use of Docker Compose to build and run a local api and a local database.

## Commands

For a validator stack against a local hardhat network:

- `make local-up`
- `make local-down`

For a validator stack against deployed enviroments:

- `make staging-up` / `make staging-down`


## Configuration

Note that for deployed enviroments, there're two relevant configuration files in each folder `deployed/<enviroment>`:

- `.env_validator` which allows to configure enviroments to fill secrets for the validator, plus expand variables present in the config file. There's a `.env_validator.example` with the expected keys to be filled.
- `config.json` is the configuration file for the validator.
