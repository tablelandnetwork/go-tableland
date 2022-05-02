BEGIN;

-- ### Add chain_id to registry ###
ALTER TABLE registry ADD COLUMN chain_id bigint;

-- Set existing rows to Rinkeby & then force the new collumn to not be NULL.
UPDATE registry SET chain_id=4;
ALTER TABLE registry ALTER COLUMN chain_id SET NOT NULL;


ALTER TABLE system_acl DROP CONSTRAINT system_acl_table_id_fkey;
ALTER TABLE system_txn_receipts DROP CONSTRAINT system_txn_receipts_table_id_fkey;

ALTER TABLE registry DROP CONSTRAINT system_tables_pkey;
ALTER TABLE registry ADD PRIMARY KEY (chain_id, id);

-- ### Add chain_id to system_acl ###
ALTER TABLE system_acl ADD COLUMN chain_id bigint;
UPDATE system_acl SET chain_id=4;
ALTER TABLE system_acl ALTER COLUMN chain_id SET NOT NULL;


ALTER TABLE system_acl DROP CONSTRAINT system_acl_pkey;
ALTER TABLE system_acl ADD PRIMARY KEY (chain_id, table_id, controller);
ALTER TABLE system_acl ADD CONSTRAINT system_acl_chain_id_table_id_fkey FOREIGN KEY (chain_id, table_id) REFERENCES registry (chain_id, id);

-- TODO(jsign): same with system_txn_receipts

COMMIT;