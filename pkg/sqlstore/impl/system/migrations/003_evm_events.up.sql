CREATE TABLE IF NOT EXISTS system_evm_events (
    chain_id INTEGER NOT NULL,
    event_json TEXT NOT NULL,
    address TEXT NOT NULL,
    topics TEXT NOT NULL,
    data BLOB NOT NULL
    block_number INTEGER NOT NULL,
    tx_hash TEXT NOT NULL,
    tx_index INTEGER NOT NULL,
    block_hash TEXT NOT NULL,
    index INTEGER NOT NULL,

    PRIMARY KEY(chain_id, tx_hash, index),
);
CREATE INDEX system_evm_events_chain_id_txn_hash on system_evm_events(chain_id, txn_hash);