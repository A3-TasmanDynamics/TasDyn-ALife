-- A SELECT-only role for the Admin Panel's database browser
-- (src/website/internal/dbbrowser). Run as a superuser (e.g. postgres):
--
--   psql -U postgres -d alife_db -f database/fixes/2026-09-30_readonly_role.sql
--
-- then set, in the website's environment (.env locally):
--
--   DATABASE_READONLY_URL=postgres://alife_readonly:<password>@127.0.0.1:5432/alife_db
--
-- Change the password below first. The role can only read, and can't read
-- the secret columns (session and link-code hashes, idempotency tokens), so
-- the browser's console needs no extra restrictions when it's used.

DO $$ BEGIN
    CREATE ROLE alife_readonly LOGIN PASSWORD 'CHANGE-ME' NOSUPERUSER NOCREATEDB NOCREATEROLE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER ROLE alife_readonly SET default_transaction_read_only = on;
ALTER ROLE alife_readonly SET statement_timeout = '5s';

GRANT CONNECT ON DATABASE alife_db TO alife_readonly;
GRANT USAGE ON SCHEMA public TO alife_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO alife_readonly;
-- Tables added later are readable too.
ALTER DEFAULT PRIVILEGES FOR ROLE alife_admin IN SCHEMA public GRANT SELECT ON TABLES TO alife_readonly;

-- Secrets: take table-level SELECT away and grant every column except the secret one.
DO $$
DECLARE
    t text; secret text; cols text;
BEGIN
    FOR t, secret IN VALUES ('web_sessions', 'token_hash'), ('discord_link_codes', 'code'),
                            ('applied_request_tokens', 'token'), ('bank_transactions', 'request_token')
    LOOP
        EXECUTE format('REVOKE SELECT ON %I FROM alife_readonly', t);
        SELECT string_agg(quote_ident(column_name), ', ') INTO cols
        FROM information_schema.columns WHERE table_schema = 'public' AND table_name = t AND column_name <> secret;
        EXECUTE format('GRANT SELECT (%s) ON %I TO alife_readonly', cols, t);
    END LOOP;
END $$;
