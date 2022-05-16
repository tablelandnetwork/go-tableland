BEGIN;

ALTER TABLE system_txn_processor ADD COLUMN chain_id bigint;
UPDATE system_txn_processor SET chain_id=69;
ALTER TABLE system_txn_processor ALTER COLUMN chain_id SET NOT NULL;
ALTER TABLE system_txn_processor ADD PRIMARY KEY (chain_id);

COMMIT;