DO $$
DECLARE query text;
begin
FOR query IN
    SELECT 'ALTER TABLE ' || table_name || ' RENAME TO _4' || table_name || ';' 
FROM information_schema.tables 
WHERE table_name ~ '^_[0-9]+$' AND
      table_type = 'BASE TABLE'
LOOP
   EXECUTE query;
END LOOP;

end;
$$