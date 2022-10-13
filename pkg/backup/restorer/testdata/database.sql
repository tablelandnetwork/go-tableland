-- This is used to create the database.db.zst test file

CREATE TABLE a (a int);
INSERT INTO a VALUES (1);

CREATE TABLE system_pending_tx (
    chain_id INTEGER NOT NULL,
    address TEXT NOT NULL,
    hash TEXT NOT NULL,
    nonce INTEGER NOT NULL,
    bump_price_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER,

    PRIMARY KEY(chain_id, address, nonce)
);

INSERT INTO system_pending_tx VALUES (1, 'address', 'hash', 1, 0, strftime('%s', 'now'), strftime('%s', 'now'));

CREATE TABLE IF NOT EXISTS system_id (
    id TEXT NOT NULL,
    PRIMARY KEY(id)
);

INSERT INTO system_id VALUES ('node id');