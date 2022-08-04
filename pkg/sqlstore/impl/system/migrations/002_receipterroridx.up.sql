ALTER TABLE system_txn_receipts ADD error_event_idx INTEGER;

UPDATE system_txn_receipts SET error_event_idx=0 WHERE error <> '';