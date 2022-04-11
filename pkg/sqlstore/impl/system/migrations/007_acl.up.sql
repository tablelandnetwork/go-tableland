BEGIN;

CREATE TABLE IF NOT EXISTS system_acl (
    table_id NUMERIC(60) REFERENCES registry(id),
    controller TEXT NOT NULL,
    privileges VARCHAR[] NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP,
    PRIMARY KEY(table_id, controller)
);

INSERT INTO system_acl (controller, table_id, privileges) SELECT controller, id, '{a, w, d}' FROM registry;

COMMIT;
