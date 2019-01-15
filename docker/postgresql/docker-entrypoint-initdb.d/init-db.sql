CREATE USER webdav WITH PASSWORD 'password';
CREATE DATABASE webdav WITH OWNER webdav;

\connect webdav;

\connect webdav webdav;