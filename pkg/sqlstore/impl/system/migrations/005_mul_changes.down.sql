ALTER TABLE system_tables DROP COLUMN name;
ALTER TABLE system_tables DROP COLUMN description;
ALTER TABLE system_tables DROP COLUMN structure;
ALTER TABLE system_tables DROP COLUMN id;
ALTER TABLE system_tables ADD COLUMN type VACHAR(32) DEFAULT '';
ALTER TABLE system_tables ADD COLUMN uuid UUID PRIMARY KEY;
