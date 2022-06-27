CREATE TABLE IF NOT EXISTS registry (
    id INTEGER NOT NULL,
    structure TEXT NOT NULL,
    controller TEXT NOT NULL,
    prefix TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    chain_id INTEGER,

    PRIMARY KEY(chain_id, id),
    CHECK(id >= 0)
);
CREATE INDEX registry_chain_id_controller on registry(chain_id, controller);

CREATE TABLE IF NOT EXISTS system_acl (
    table_id INTEGER NOT NULL,
    controller TEXT NOT NULL,
    privileges INT NOT NULL,
    chain_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER,

    PRIMARY KEY(chain_id, table_id, controller),
    FOREIGN KEY(chain_id, table_id) REFERENCES registry(chain_id, id)
);

CREATE TABLE IF NOT EXISTS system_controller (
    chain_id INTEGER NOT NULL,
    table_id INTEGER NOT NULL,
    controller TEXT NOT NULL,

    PRIMARY KEY(chain_id, table_id),
    CHECK (controller != '0x0000000000000000000000000000000000000000'),
    FOREIGN KEY(chain_id, table_id) REFERENCES registry(chain_id, id)
);

CREATE TABLE IF NOT EXISTS system_txn_receipts (
    chain_id INTEGER NOT NULL,
    block_number INTEGER NOT NULL,
    index_in_block INTEGER NOT NULL,
    txn_hash TEXT NOT NULL,
    error TEXT,
    table_id INTEGER,

    PRIMARY KEY(chain_id, block_number, index_in_block)
);
CREATE UNIQUE INDEX system_txn_receipts_txn_hash on system_txn_receipts(chain_id, txn_hash);

CREATE TABLE IF NOT EXISTS system_txn_processor (
    chain_id INTEGER PRIMARY KEY NOT NULL,
    block_number INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS system_pending_tx (
    chain_id INTEGER NOT NULL,
    address TEXT NOT NULL,
    hash TEXT NOT NULL,
    nonce INTEGER NOT NULL,
    bump_price_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER,

    PRIMARY KEY(chain_id, address, nonce)
);