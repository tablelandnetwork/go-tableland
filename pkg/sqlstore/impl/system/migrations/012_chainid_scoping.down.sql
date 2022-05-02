BEGIN;

ALTER TABLE registry DROP CONSTRAINT registry_pkey;
ALTER TABLE registry ADD PRIMARY KEY (id);

ALTER TABLE registry DROP COLUMN chain_id bigint;

COMMIT;





