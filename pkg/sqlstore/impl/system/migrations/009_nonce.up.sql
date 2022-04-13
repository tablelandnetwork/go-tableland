CREATE TABLE IF NOT EXISTS system_nonce (
    network varchar not null,
    address varchar not null,
    nonce bigint not null,
    PRIMARY KEY (network, address)
);

CREATE TABLE IF NOT EXISTS system_pending_tx (
    network varchar not null,
    address varchar not null,
    hash varchar not null, 
    nonce bigint not null
);

