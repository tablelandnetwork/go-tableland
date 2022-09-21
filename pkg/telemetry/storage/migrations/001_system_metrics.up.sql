CREATE TABLE IF NOT EXISTS system_metrics(
    version INTERGER NOT NULL,
    timestamp INTEGER NOT NULL,
    type INTEGER NOT NULL,
    payload TEXT NOT NULL,
    published INTEGER NOT NULL
);