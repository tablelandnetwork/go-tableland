CREATE TABLE IF NOT EXISTS system_controller (
    chain_id bigint not null,
    table_id NUMERIC(60),
    controller varchar not null,
    PRIMARY KEY (chain_id, table_id),
    CONSTRAINT fk_controller_registry FOREIGN KEY (chain_id, table_id) REFERENCES registry (chain_id, id)
);

