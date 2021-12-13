CREATE TABLE IF NOT EXISTS system_tables (
    uuid UUID PRIMARY KEY,
    controller CHAR(42) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
