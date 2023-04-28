ALTER TABLE system_txn_receipts ADD table_ids TEXT;

UPDATE system_txn_receipts SET table_ids=cast(table_id as text) WHERE table_id is not null;