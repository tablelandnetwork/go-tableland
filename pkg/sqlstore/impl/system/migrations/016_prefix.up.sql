ALTER TABLE registry RENAME COLUMN name TO prefix;

DO $$
DECLARE query text;
begin
FOR query IN
    SELECT 'ALTER TABLE ' || t.table_name || ' RENAME TO ' || r.prefix || t.table_name || ';' 
FROM information_schema.tables as t
INNER JOIN registry as r on ('_' || r.chain_id || '_' || r.id)=t.table_name
WHERE t.table_name ~ '^_[0-9]+_[0-9]+$' AND
      t.table_type = 'BASE TABLE'
LOOP
   EXECUTE query;
END LOOP;

end;
$$
