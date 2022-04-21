CREATE TABLE IF NOT EXISTS system_txn_receipts (
    id bigserial,
    chain_id bigint not null,
    block_number bigint not null,
    txn_hash text not null,
    error text,
    table_id numeric(60) REFERENCES registry(id)
);

