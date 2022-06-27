CREATE TABLE IF NOT EXISTS registry (
    id INTEGER NOT NULL,
    structure TEXT NOT NULL,
    controller TEXT NOT NULL,
    prefix TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    chain_id INTEGER,

    PRIMARY KEY(chain_id, id),
    CHECK(id >= 0)
);

CREATE TABLE IF NOT EXISTS system_acl (
    table_id INTEGER NOT NULL,
    controller TEXT NOT NULL,
    privileges INT NOT NULL,
    chain_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT,

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