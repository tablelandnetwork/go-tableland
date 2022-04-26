CREATE TABLE IF NOT EXISTS system_pending_tx (
    chain_id bigint not null,
    address varchar not null,
    hash varchar not null, 
    nonce bigint not null,
    created_at timestamp default now() not null,
    PRIMARY KEY (chain_id, address, nonce)
);

