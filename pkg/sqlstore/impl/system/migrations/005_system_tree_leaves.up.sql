CREATE TABLE IF NOT EXISTS system_tree_leaves (
    prefix TEXT NOT NULL,
    chain_id INTEGER NOT NULL,
    table_id INTEGER NOT NULL,
    block_number INTEGER NOT NULL,
    leaves BLOB NOT NULL,
    
    PRIMARY KEY(chain_id, table_id, block_number)
);
