![image](https://user-images.githubusercontent.com/6136245/153219831-53b05f19-1ac2-4523-b564-0686e2078d4d.png)
[![Go Reference](https://pkg.go.dev/badge/github.com/textileio/go-tableland.svg)](https://pkg.go.dev/github.com/textileio/go-tableland) [![Go Report Card](https://goreportcard.com/badge/github.com/textileio/go-tableland)](https://goreportcard.com/report/github.com/textileio/go-tableland)
<h1 align="center">Tableland Validator</h1>

This repository contains the Go implementation of a Tableland validator.

# What is a validator?

Validators are the execution unit/actors of the protocol.

They have the following responsibilities:

- Listen to on-chain events to materialize Tableland-compliant SQL queries in a database engine (currently, SQLite by default).
- Serve read-queries (e.g: `SELECT * FROM foo_69_1`) to the external world.
- Relay write-queries to the `Registry` SC on behalf of users.

> 💡 The responsibilities of the validator will continue to change as the Tableland protocol evolves. In the future, validators will have more responsibilities in the network.

# Where does the validator fit in the network?

The following is a diagram that describes at a high level the interaction between the validator, EVM-chains, and the external world:

![image](https://user-images.githubusercontent.com/6136245/180085125-3093401c-5ef0-449e-93b7-c323fd0bfbce.png)

To understand better the usual work mechanics of the validator, let’s go through a typical use-case where a user mints a table, adds data to the table, and reads it:

1- The user will mint a table in the `Registry` smart contract.

2- The `Registry` contract will emit a `CreateTable` event containing the `CREATE TABLE` statement as extra data.

3- Validators will detect the new event and execute the `CREATE TABLE` statement.

4- The user will call the `runSQL` method in the `Registry` smart contract, with mutating statements such as `INSERT INTO ...`.

5- The `Registry` contract, as a result of that call, will emit a `RunSQL` event that contains the `INSERT TABLE` statement as extra data.

6- The validators will detect the new event and execute the mutating query in the corresponding table.

7- The user can query the `/query?statement=...` REST endpoint of the validator to execute read-queries (e.g: `SELECT * FROM ...`), to see the materialized result of its interaction with the SC.

> 💡 The description above is optimized to understand the general mechanics of the validator. Minting tables, and executing mutating statements also imply more work both at the SC and validator levels (e.g: ACL enforcing); we’re skipping them here.

The validator detects the smart contract events using an EVM node API (e.g: `geth` node), which can be self-hosted or served by providers (e.g: Alchemy, Infura, etc).

# How can I run a validator?

Soon we'll be creating full documentation on how to configure and run a validator, stay tuned.
If you're interested in running a Tableland validator, please [fill this form](https://hhueol4i6vp.typeform.com/to/gkcyeA22).

# Building from source

You can build the validator binary from source by running: `make build-api`.

# Run with docker-compose

The repository contains in the `docker` folder a complete docker-compose setup to run a validator.

Soon we'll publish dedicated documentation with instructions on how to run it.

# Tools

The `cmd/toolkit` is a CLI which contain useful commands:

- `gaspricebump`: Bumps gas price for a stuck transaction
- `sc`: Offers smart sontract calls
- `wallet`: Offers wallet utilites

# Contributing

Pull requests and bug reports are very welcome.

Feel free to get in touch by:

- Opening an issue.
- Joining our [Discord server](https://discord.gg/dc8EBEhGbg).

# License

MIT AND Apache-2.0, © 2021-2022 Tableland Network Contributors
