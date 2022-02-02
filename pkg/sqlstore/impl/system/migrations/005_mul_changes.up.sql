ALTER TABLE system_tables DROP COLUMN type;
ALTER TABLE system_tables DROP COLUMN uuid;
ALTER TABLE system_tables ADD COLUMN id BIGINT PRIMARY KEY CHECK (id >= 0);
ALTER TABLE system_tables ADD COLUMN structure CHAR(64);
ALTER TABLE system_tables ADD COLUMN description VARCHAR(100);
ALTER TABLE system_tables ADD COLUMN name VARCHAR(50);

