# Tableland Validator

[![Review](https://github.com/tablelandnetwork/go-tableland/actions/workflows/review.yml/badge.svg)](https://github.com/tablelandnetwork/go-tableland/actions/workflows/review.yml)
[![Test](https://github.com/tablelandnetwork/go-tableland/actions/workflows/test.yml/badge.svg)](https://github.com/tablelandnetwork/go-tableland/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/textileio/go-tableland.svg)](https://pkg.go.dev/github.com/textileio/go-tableland)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/go-tableland)](https://goreportcard.com/report/github.com/textileio/go-tableland)
[![License](https://img.shields.io/github/license/tablelandnetwork/go-tableland.svg)](./LICENSE)
[![Release](https://img.shields.io/github/release/tablelandnetwork/go-tableland.svg)](https://github.com/tablelandnetwork/go-tableland/releases/latest)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg)](https://github.com/RichardLitt/standard-readme)

> Go implementation of the Tableland database—run your own node, handling on-chain mutating events and serving read-queries.

## Table of Contents

- [Background](#background)
  - [What is a validator?](#what-is-a-validator)
  - [Validator and network relationship](#validator-and-network-relationship)
  - [Running a validator](#running-a-validator)
- [Usage](#usage)
  - [System requirements](#system-requirements)
  - [Firewall configuration](#firewall-configuration)
  - [System prerequisites](#system-prerequisites)
  - [Run the validator](#run-the-validator)
  - [Docker Compose setup](#docker-compose-setup)
  - [Backups and other routines](#backups-and-other-routines)
- [Development](#development)
  - [Configuration](#configuration)
- [Contributing](#contributing)
- [License](#license)

## Background

`go-tableland` is a Go language implementation of a Tableland node, enabling developers and service providers to run nodes on the Tableland network and host databases for web3 users and applications. Note that the Tableland protocol is currently in open beta, so node operators have the opportunity to be one of the early network adopters while the responsibilities of the validator will continue to change as the Tableland protocol evolves.

### What is a validator?

Validators are the execution unit/actors of the protocol.

They have the following responsibilities:

- Listen to on-chain events to materialize Tableland-compliant SQL queries in a database engine (currently, SQLite by default).
- Serve read-queries (e.g., `SELECT * FROM foo_69_1`) to the external world.

> In the future, validators will have more responsibilities in the network.

### Validator and network relationship

The following diagram describes a high level interaction between the validator, EVM chains, and the external world:

<p align="center">
  <img src="https://user-images.githubusercontent.com/13358940/249310798-a65732b9-a48b-4547-bc4d-71af8bd4e09f.png" width='80%'/>
</p>

To better understand the usual mechanics of the validator, let’s go through a typical use case where a user mints a table, adds data to the table, and reads from it:

1. The user will mint a table (ERC721) from the Tableland `Registry` smart contract on a supported EVM chain.
2. The `Registry` contract will emit a `CreateTable` event containing the `CREATE TABLE` statement as extra data.
3. Validators will detect the new event and execute the `CREATE TABLE` statement.
4. The user will call the `mutate` method in the `Registry` smart contract, with mutating statements such as `INSERT INTO ...`, `UPDATE ...`, `DELETE FROM ...`, etc.
5. The `Registry` contract, as a result of that call, will emit a `RunSQL` event that contains the mutating SQL statement as extra data.
6. The validators will detect the new event and execute the mutating query in the corresponding table, assuming the user has the right permissions (e.g., table ownership and/or smart contract defined access controls).
7. The user can query the `/query?statement=...` REST endpoint of the validator to execute read-queries (e.g., `SELECT * FROM ...`), to see the materialized result of its interaction with the smart contract.

> The description above is optimized to understand the general mechanics of the validator. Minting tables and executing mutating statements also imply more work both at the smart contract and validator levels (e.g., ACL enforcing), which are being omitted here for simplicity sake.

The validator detects the smart contract events using an EVM node API (e.g., `geth` node), which can be self-hosted or served by providers (e.g., Alchemy, Infura, etc).

If you're curious about Tableland network growth, eager to contribute, or interested in experimenting, we encourage you to try running a validator. To get started, follow the step-by-step instructions provided below. We appreciate your interest and welcome any questions or feedback you may have during the process; stay tuned for updates and developments in our [Discord](https://tableland.xyz/discord) and [Twitter](https://twitter.com/tableland).

For projects that want to _use_ the validator API, Tableland [maintains a public gateway](https://docs.tableland.xyz/gateway-api) that can be used to query the network.

### Running a validator

Running a validator only involves running a single process. Since we use SQLite as the default database engine, it is embedded and has many advantages:

- There’s no separate process for the database.
- There’s no inter-process communication between the validator and the database.
- There’s no separate configuration or monitoring needed for the database.

We provide everything you need to run a validator with a single command using a docker-compose setup. This will automatically build everything from the source code, making it platform-independent since most OSes support docker. The build process is also dockerized, so node operators don’t need to worry about installing compilers or similar.

If you like creating your own setup (e.g., run raw binaries, use systemd, k8, etc.), we’re also planning to automate versioned Docker images or compiled executables. If there are other setups you're interested in, feel free to let us know or even share your own setup.

The [Docker Compose setup](#docker-compose-setup) section below describes how to run a validator in more detail, including:

- Folder structure.
- Configuration files.
- Where the state of the validator lives.
- Baked in observability stack (i.e., Prometheus + Grafana with dashboard).
- Optional `healthbot` process to have an end-to-end (e2e) healthiness check of the validator.

Reviewing this section is _strongly_ recommended but not strictly necessary.

## Usage

### System requirements

Currently, we recommend running the validator on a machine that has at least:

- 4 vCPUs.
- 8GiB of RAM.
- SSD disk with 10GiB of free space.
- Reliable and fast internet connection.
- Static IP.

Hardware requirements might change with time, but this setup is probably over provisioned in the current state. We’re planning to do a stress testing benchmark suite to understand and predict the behavior of the validator under different loads to have more data about potential future recommended system requirements.

### Firewall configuration

If you’re behind a firewall, you should open ports `:8080` or `:443`, depending on if you run with TLS certificates. By default, TLS is not required, thus, expecting `:8080` to be open to the external world.

### System prerequisites

There are two prerequisites for running a validator:

- Install host-level dependencies.
- Get EVM node API keys.

Tableland has two separate networks:

- `mainnet`: this network syncs mainnet EVM chains (e.g., Ethereum mainnet, Arbitrum mainnet, etc.).
- `testnet`: this network is syncing testnet EVM chains (e.g., Ethereum Sepolia, Arbitrum Goerli, etc.).

This guide will focus on running the validator in the `mainnet` network.

We do this for two reasons:

- The `mainnet` network is the most stable one and is also where we want the most number of validators.
- We can provide concrete file paths related to `mainnet` and avoid being abstract.

We’ll also explain how to run a validator using Alchemy as a provider for the EVM node API the validator will use. The configuration will be analogous if you use self-hosted nodes or other providers.

#### Install host-level dependencies

To run the provided docker-compose setup, you’ll need to have installed:

- `git`: [Installation guide](https://github.com/git-guides/install-git).
- `docker` with the Compose plugin: [The default Docker engine installation](https://docs.docker.com/engine/install/) already includes the Compose plugin (i.e., `docker compose` command).

Note that there’s no need for a particular `Go` installation since binaries are compiled within a docker container containing the correct Go compiler versions. Despite not being strictly necessary, creating a separate user in the host is usually recommended to run the validator.

#### Create EVM node API keys

The current setup needs one API key per supported chain. The default setup expects Alchemy keys for the following: Ethereum, Optimism, Arbitrum One, and Polygon. Ankr is used for Filecoin, and QuickNode for Arbitrum Nova. But, you are free to use a self-hosted node or another provider that supports the targeted chains.

To get your Alchemy keys, create an [Alchemy](https://alchemy.com) account, log in, and follow these steps:

1. Create one app for each chain using the `+ Create App` button.
2. You’ll see one row per chain—click the `View Key` button and copy/save the `API KEY`.

To get your Ankr Filecoin Keys, create an [Ankr](http://ankr.com) account, log in, and do the following:

1. Create a Filecoin endpoint.
2. You should be able to have access to your API key.

To get your QuickNode Arbitrum Nova key, create a [QuickNode](https://quicknode.com) account, log in, and follow these steps:

1. Create an endpoint.
2. Select Arbitrum Nova Mainnet.
3. When you finish the wizard, you should be able to have access to your API key.

### Run the validator

Now that you have installed the host-level dependencies, have one wallet per chain, and provider (Alchemy, Ankr, and/or QuickNode) API keys, you’re ready to configure the validator and run it.

#### 1. Clone the `go-tableland` repository

Navigate to the folder where you want to clone the repository and run:

```sh
git clone https://github.com/tablelandnetwork/go-tableland.git
```

> Running the `main` branch should always be safe since it’s the exact code that the public validator is running. We recommend this approach since we’re moving quickly with features and improvements but expect soon to be better guided by official releases.

#### 2. Configure your secrets in `.env` files

You must configure each EVM account's private keys and EVM node provider API keys into the validator secrets:

1. Create a `.env_validator` file in `docker/deployed/mainnet/api` folder—an example is provided with `.env_validator.example`.
2. Add the following to `.env_validator` (as noted, this focuses on mainnet configurations but could be generally replicated for testnet support):

```txt
VALIDATOR_ALCHEMY_ETHEREUM_MAINNET_API_KEY=<your ethereum mainnet alchemy key>
VALIDATOR_ALCHEMY_OPTIMISM_MAINNET_API_KEY=<your optimism mainnet alchemy key>
VALIDATOR_ALCHEMY_ARBITRUM_MAINNET_API_KEY=<your arbitrum mainnet alchemy key>
VALIDATOR_ALCHEMY_POLYGON_MAINNET_API_KEY=<your polygon mainnet alchemy key>
VALIDATOR_ANKR_FILECOIN_MAINNET_API_KEY=<your filecoin mainnet ankr key>
VALIDATOR_QUICKNODE_ARBITRUM_NOVA_MAINNET_API_KEY=<your arbitrum nova mainnet quicknode key>
```

> Note: the `METRICS_HUB_API_KEY` variable is optional and can be left empty. It's a service (`cmd/metricshub`) that aggregates metrics like `git summary` and pushes them to centralized infrastructure ([GCP Cloud Run](https://cloud.google.com/run)) managed by the core team. If you'd like to have your validator push metrics to this hub, please reach out to the Tableland team, and we may make it available to you. However, this process will further be decentralized in the future and remove this dependency entirely.

1.  Tune the `docker/deployed/mainnet/api/config.json` :

    1.  Change the `ExternalURIPrefix` configuration attribute into the DNS (or IP) where your validator will be serving external requests.
    2.  In the `Chains` section, only leave the chains you’ll be running; remove any chain entries you do not wish to support.

        <details> 
          <summary>Reference: example entry</summary>

        ```json
        {
          "Name": "Ethereum Mainnet",
          "ChainID": 1,
          "Registry": {
            "EthEndpoint": "wss://eth-mainnet.g.alchemy.com/v2/${VALIDATOR_ALCHEMY_ETHEREUM_MAINNET_API_KEY}",
            "ContractAddress": "0x012969f7e3439a9B04025b5a049EB9BAD82A8C12"
          },
          "EventFeed": {
            "ChainAPIBackoff": "15s",
            "NewBlockPollFreq": "10s",
            "MinBlockDepth": 1,
            "PersistEvents": true
          },
          "EventProcessor": {
            "BlockFailedExecutionBackoff": "10s",
            "DedupExecutedTxns": true,
            "WebhookURL": "https://discord.com/api/webhooks/${VALIDATOR_DISCORD_WEBHOOK_ID}/${VALIDATOR_DISCORD_WEBHOOK_TOKEN}"
          },
          "HashCalculationStep": 150
        }
        ```

        </details>

2.  Create a `.env_grafana` file in the `docker/deployed/mainnet/grafana` folder—an example is provided with `.env_grafana.example`.
3.  Add the following to `.env_grafana`:

```txt
GF_SECURITY_ADMIN_USER=<user name you'd like to login intro grafana>
GF_SECURITY_ADMIN_PASSWORD=<password of the user>
```

> Note: the `GF_SERVER_ROOT_URL` variable is optional and can be left empty. By default, Grafana is hosted locally at `http://localhost:3000`.

That’s it...your validator is now configured!

It's worthwhile to review the `config.json` file to see how the environment variables configured in the `.env` files inject these secrets into the validator configuration. Also, note how supporting more chains only requires adding an extra entry in the `Chains`, so it's straightforward to add support for any of the supported `testnets` of each `mainnet` chain. Note that adding a _new_ `mainnet` chain that's not yet supported by the network is not possible as this requires the core Tableland protocol to separately deploy a `Registry` smart contract in order to enable new chain support. This is performed on a case-by-case basis, so please reach out to the Tableland team if you'd like support for a new `mainnet` chain.

#### 3. Run the validator

To run the validator, move to the `docker` folder and run the following:

```sh
make mainnet-up
```

Some general comments and tips:

- The first time you run this, it can take some time since you’ll have a cold cache regarding images and dependencies in Docker; subsequent runs will be quite fast.
- You can inspect the general health of containers with `docker ps`.
- You can tail the logs with `docker logs docker-api-1 -f`.
- You can tear down the stack with `make mainnet-down`.

> The default docker-compose setup has a baked-in observability substack with Prometheus and Grafana. You can learn more about this in the next section.

While the validator is syncing, you might see the logs are generated rather quickly. In the `docker/deployed/mainnet/api/database.db`, you should expect that the SQLite database will start to grow in size.

### Docker Compose setup

The docker-compose setup can feel a bit magical, so in this section, we’ll explain the setup's folder structure and important considerations. Remember that you don’t need to understand this section to run a validator, but knowing how things work is highly recommended.

#### Architecture and port bindings

When you run `make mainnet-up`, you’re running the following stack:

<p align="center">
  <img src="https://user-images.githubusercontent.com/13358940/249326369-b94c12b9-6550-49c9-92b3-5e662c8bcd72.png" width='80%'/>
</p>

If you’re running the validator, you’ll see these four containers running with `docker ps`.

There’re two main port binding groups:

- `:8080` and `:443` to the `api` container (the validator), depending if you have configured TLS in the validator.
- `:3000` to the `grafana` container to access the Grafana dashboard. Remember that if you want to access to Grafana from the external world, you’ll have to configure your firewall.

Regarding the containers:

- `api` is the container running the validator.
- `healthbot` is an optional container to have an e2e daemon checking the healthiness of the full write-query transaction and events execution. More about this in the [Healthbot section](https://www.notion.so/Validator-documentation-9f0cc2abf424410c8659fa939ed5095e?pvs=21).
- `grafana` and `prometheus` are part of the observability stack, allowing a fully-featured Grafana dashboard that provides useful live information about the validator. There's more information about this in the [Observability section](#observability-stack).

#### Folder structure

The `docker/deployed/mainnet` folder contains one folder per process that it’s running:

- `api` folder: contains all the relevant secrets, configuration and state of the validator.
  - `config.json` file: the full configuration file of the validator.
  - `.env_validator` file: contains secrets that are injected in the `config.json` file.
  - `database.db*` files: when you run the validator, you’ll see these files, which are the SQLite database of the validator (running in [WAL](https://www.sqlite.org/wal.html) mode).
- `grafana` and `prometheus` folders: contain any state from these daemons. For example, Grafana can include alerts or settings customizations, and Prometheus has the time-series database, so whenever you reset the container, it will keep historical data.
- `healthbot` folder: contains secrets and configuration for the healthbot.

From an operational point of view, you usually don’t have to touch these folders apart from the `api/config.json` or `api/.env_validator` if you want to change something about the validator configuration or secrets. The Prometheus setup has a default 15 days retention time for the time series data, so the database size should be automatically bounded.

#### Configuration files

The validator configuration is done via a JSON file located at `deployed/mainnet/api/config.json`.

This file contains general and chain-specific configuration, such as desired listening ports, gateway configuration, log level configuration, and chain-specific configuration, including name, chain ID, contract address, wallet private keys, and EVM node API endpoints.

The provided configurations in each `deployed/<environment>` already have everything needed for the environment and other recommended values. The environment variable expansion parts of the `config.json` file, such as secrets and other attributes in the `.env_validator` file, were explained in the [secret configuration section](2-configure-your-secrets-in-env-files) above. For example, the `VALIDATOR_ALCHEMY_ETHEREUM_MAINNET_API_KEY` variable configured in `.env_validator` expands a `${VALIDATOR_ALCHEMY_ETHEREUM_MAINNET_API_KEY}` present in the `config.json` file. If you want to use a self-hosted Ethereum mainnet node API or another provider, you can edit the `config.json` file in the `EthEndpoint` endpoint. This same logic applies to every possible configuration in the validator.

#### Observability stack

As mentioned earlier, the default docker-compose setup provides a fully configured observability stack by running Prometheus and Grafana.

This setup configures the scrape endpoints in Prometheus to pull metrics from the validator and data sources dashboard for Grafana. These automatically bound configuration files are in `docker/observability/(grafana|prometheus)` folders. They are not part of the state of the processes. This is intentional so that, for example, the dashboard is part of the `go-tableland` repository, and you’ll get automatic dashboard upgrades while is being improved or extended.

After you spin up the validator, you can go to `http://localhost:3000` and access the Grafana setup. Recall that you configured the credentials in the `.env_grafana` file in `docker/deployed/mainnet/grafana`.

If you browse the existing dashboards, you should see an existing _Validator_ dashboard that should look like the following, which aggregates all metrics that the validator generates:

<p align="center">
  <img src="https://user-images.githubusercontent.com/13358940/249328422-3d727309-42b8-4ffc-9f2f-bcfed6b5d398.png" width='80%'/>
</p>

#### Healthbot (optional)

The `healthbot` daemon is an optional feature of the docker-compose stack and is _only_ needed if you support a testnet network; it's disabled by default.

The main goal of `healthbot` is to test e2e in order to see if the validator is running correctly:

- For every configured chain, it executes a write statement to Tableland smart contract to increase a counter value in a pre-minted table that is owned by the validator.
- It waits to see if the increased counter in the target table was materialized in the table, thus, signaling that:
  - The transaction with the `UPDATE` statement was correctly sent to the chain.
  - The transaction was correctly minted in the target blockchain.
  - The event for that `UPDATE` was detected and processed by the validator
  - A `SELECT` statement reading that table should read the increased counter in the target table.

In short, it tests most of the processing healthiness of the validator. For each of the target chains, you should mint a table with the following statement:

```sql
CREATE TABLE healthbot_{chainID} (counter INTEGER);
```

This would result in having four tables—one per chain:

- `healthbot_11155111_{tableID}` (Ethereum Sepolia)
- `healthbot_420_{tableID}` (Optimism Goerli)
- `healthbot_421613_{tableID}` (Arbitrum Goerli)
- `healthbot_80001_{tableID}` (Polygon Mumbai)
- `healthbot_314159_{tableID}` (Filecoin Calibration)

You should create a file `.env_healthbot` in the `docker/deployed/testnet/healthbot` folder with the following content (an example is provided with `.env_healthbot.example`):

```txt
HEALTHBOT_ETHEREUM_GOERLI_TABLE=healthbot_5_{tableID}
HEALTHBOT_OPTIMISM_GOERLI_TABLE=healthbot_420_{tableID}
HEALTHBOT_ARBITRUM_GOERLI_TABLE=healthbot_421613_{tableID}
HEALTHBOT_POLYGON_MUMBAI_TABLE=healthbot_80001_{tableID}
HEALTHBOT_FILECOIN_CALIBRATION_TABLE=healthbot_314159_{tableID}
```

Finally, edit the `docker/deployed/testnet/healthbot/config.json` file `Target` attribute with the public DNS where your validator is serving to the external world. This is the endpoint where the healthbot will be making the healthiness probes. Since running the `healthbot` requires custom tables to be minted, it’s disabled by default.

To enable running the `healthbot`, you should run the following `make testnet-up` with the `HEALTHBOT_ENABLED=true` environment value set:

```sh
HEALTHBOT_ENABLED=true make testnet-up
```

After a few minutes, you should see the `HealthBot -e2e check` section of the Grafana dashboard populated:

<p align="center">
  <img src="https://user-images.githubusercontent.com/13358940/249330376-53afd85e-693b-47d4-b877-9463a10af135.png" width='80%'/>
</p>

#### Pruning docker images (optional)

Removing old docker images from time to time may be beneficial to avoid unnecessary disk usage. You can set up a `cron` rule to do that automatically. For example, you could do the following:

1. Run `crontab -e`.
2. Add the rule: `0 0 * * FRI /usr/bin/docker system prune --volumes -f  >> /home/validator/cronrun 2>&1`

### Backups and other routines

All validators are equipped with a backup scheduler that runs a background routine that executes a backup process of the SQLite database file at a configurable regular frequency. Besides the main backup of the database, the `Backuper` process executes a `VACUUM` process in the backup file and compresses it with `zstd`.

#### How the backup process works

The backup process called `Backuper` takes a backup of `SQLite` database file and stores it in a local directory relative to where the database is stored.

The process uses the [SQLite Backup API](https://www.sqlite.org/c3ref/backup_finish.html) provided by [mattn/go-sqlite3](https://pkg.go.dev/github.com/mattn/go-sqlite3#SQLiteBackup). It is a full backup in a single step. Right now, the database is small enough not to worry about locking and how long it takes, but an incremental backup approach may be needed when as the database grows in the future.

#### How the scheduler works

The scheduler ticks at a regular interval defined by the `Frequency` config. It is important to mention that the time it runs is relative to the epoch time. That means, as the validator becomes operational and healthy after a deployment, it will start a backup routine in the next timestamp multiple of `Frequency` relative to epoch. That allows having backup files evenly distributed according to timestamp.

#### Vacuum

After the backup is finished, it executes the `VACUUM` SQL statement in the backup database to remove any unused rows and reduce the database file. This process may take a while, but it's expected since there shouldn't be any other connections to the backup database at this point.

#### Compression

After **vacuum**, we shrink the database even further by compressing it using the [zstd](http://facebook.github.io/zstd/) algorithm implemented by [compress](https://github.com/klauspost/compress) library.

#### Pruning

We don't keep all backup files around—at the end, we remove any files exceeding the backup's `KeepFiles` config, located in `cmd/api/config.go`. The default value is `5`.

#### Filename convention

The backup files follow the pattern: `tbl_backup_{{TIMESTAMP}}.db.zst`. For example, it should resemble the following: `tbl_backup_2022-08-25T20:00:00Z.db.zst`.

#### Decompressing the file

If you're on Linux or Mac, you should have `unzstd` installed out of the box. For example, run `unzstd tbl_backup_2022-08-25T20:00:00Z.db.zst` (replace with your file name) to decompress the compressed database file.

#### Metrics

We collect the following metrics from the process through **logs**:

```go
Timestamp.             time.Time
ElapsedTime            time.Duration
VacuumElapsedTime      time.Duration
CompressionElapsedTime time.Duration
Size                   int64
SizeAfterVacuum        int64
SizeAfterCompression   int64
```

Additionally, we collect the metric `tableland.backup.last_execution` through **Open Telemetry** and **Prometheus**.

#### Configs

The backup configuration files are located in the `docker/deployed/mainnet/api/config.json` file. The following is the default configuration:

```json
"Backup" : {
  "Enabled": true,       // enables the backup scheduler to execute backups
  "Dir": "backups",      // where backup files are stored relative to db
  "Frequency": 240,      // backup frequency in minutes
  "EnableVacuum": true,
  "EnableCompression": true,
  "Pruning" : {
    "Enabled": true,  // enables pruning
    "KeepFiles": 5    // pruning keeps at most `KeepFiles` backup files
  }
}
```

## Development

Get started by following the validator setup steps described above. From there, you can make changes to the codebase and run the validator locally. For a validator stack against a local Hardhat network, you can run the following from the `docker` folder:

- `make local-up`
- `make local-down`

For a validator stack against deployed staging environments, you can run:

- `make staging-up`
- `make staging-down`

### Configuration

Note that for deployed environments, there are two relevant configuration files in each folder `docker/deployed/<environment>`:

- `.env_validator`: allows you to configure environments to fill secrets for the validator, plus, expand variables present in the config file (see the `.env_validator.example` example file).
- `config.json`: the configuration file for the validator.

Besides that, you may want to configure Grafana's `admin_user` and `admin_password`. To do that, configure the `.env_grafana` file with the values of the expected keys shown in `.env_grafana.example`. This all should have been set up already but is worth noting.

## Contributing

PRs accepted. Feel free to get in touch by:

- Opening an issue.
- Joining our [Discord server](https://tableland.xyz/discord).

Small note: If editing the README, please conform to the
[standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

MIT AND Apache-2.0, © 2021-2023 Tableland Network Contributors
