CREATE TABLE IF NOT EXISTS system_evm_events (
    chain_id INTEGER NOT NULL,
    event_json TEXT NOT NULL,
    address TEXT NOT NULL,
    topics TEXT NOT NULL,
    data BLOB NOT NULL,
    block_number INTEGER NOT NULL,
    tx_hash TEXT NOT NULL,
    tx_index INTEGER NOT NULL,
    block_hash TEXT NOT NULL,
    event_index INTEGER NOT NULL,

    PRIMARY KEY(chain_id, tx_hash, event_index)
);
CREATE INDEX system_evm_events_chain_id_tx_hash on system_evm_events(chain_id, tx_hash);

CREATE TABLE IF NOT EXISTS system_evm_blocks (
    chain_id INTEGER NOT NULL,
    block_number INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,

    PRIMARY KEY(chain_id, block_number)
)