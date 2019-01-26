ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET synchronous_commit = 'off'; -- https://postgrespro.ru/docs/postgrespro/9.5/runtime-config-wal.html#GUC-SYNCHRONOUS-COMMIT
-- ALTER SYSTEM SET shared_buffers='512MB';
ALTER SYSTEM SET fsync=FALSE;
ALTER SYSTEM SET full_page_writes=FALSE;
ALTER SYSTEM SET commit_delay=100000;
ALTER SYSTEM SET commit_siblings=10;


CREATE USER webdav WITH PASSWORD 'password';
CREATE DATABASE webdav WITH OWNER webdav;

ALTER SCHEMA public OWNER TO webdav;

\connect webdav;

\connect webdav webdav;