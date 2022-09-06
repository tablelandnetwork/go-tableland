CREATE TABLE IF NOT EXISTS system_metrics(
    timestamp INTEGER NOT NULL,
    type INTEGER NOT NULL,
    payload TEXT NOT NULL,
    published INTEGER NOT NULL
);