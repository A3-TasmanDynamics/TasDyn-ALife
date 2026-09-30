-- One-off repair for players rows whose uid was stored with Arma's literal
-- quotes (e.g. '"76561198127262076"'), created before the extension began
-- unwrapping callExtension string args (UnwrapArmaString, 2026-09-29).
--
-- Safe by design -- run it with psql; it only changes a row when nothing
-- of value would be lost:
--   * a quoted row WITH an unquoted twin is deleted only if the only rows
--     referencing it are zero-balance bank_accounts (the blank-player
--     seed); anything else is left alone and reported for a human to merge.
--   * a quoted row WITHOUT a twin just has its uid unquoted in place.
-- Finally the uid CHECK constraint from schema.sql is added, which fails
-- (and rolls everything back) if any bad uid remains.
--
--   psql -U alife_admin -h 127.0.0.1 -d alife_db -f database/fixes/2026-09-29_quoted_uids.sql

BEGIN;

-- Attribute any rank_changes/staff_log rows this produces.
SELECT set_config('tasdyn.source', 'manual', true),
       set_config('tasdyn.reason', 'repair: quoted Steam64 uid (database/fixes/2026-09-29_quoted_uids.sql)', true);

DO $$
DECLARE
    bad   RECORD;
    fk    RECORD;
    n     BIGINT;
    blocking BIGINT;
    twin  BIGINT;
BEGIN
    FOR bad IN SELECT id, uid, btrim(uid, '"') AS clean FROM players WHERE uid !~ '^[0-9]{17}$' LOOP
        IF bad.clean !~ '^[0-9]{17}$' THEN
            RAISE NOTICE 'player %: uid % is not a quoted Steam64 ID -- left for manual review', bad.id, bad.uid;
            CONTINUE;
        END IF;

        SELECT id INTO twin FROM players WHERE uid = bad.clean;
        IF twin IS NULL THEN
            UPDATE players SET uid = bad.clean WHERE id = bad.id;
            RAISE NOTICE 'player %: uid unquoted in place (%)', bad.id, bad.clean;
            CONTINUE;
        END IF;

        -- Count references other than zero-balance bank accounts.
        blocking := 0;
        FOR fk IN
            SELECT tc.table_name, kcu.column_name
            FROM information_schema.table_constraints tc
            JOIN information_schema.key_column_usage kcu
              ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
            JOIN information_schema.constraint_column_usage ccu
              ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
            WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = 'players' AND ccu.column_name = 'id'
        LOOP
            IF fk.table_name = 'bank_accounts' THEN
                EXECUTE format('SELECT count(*) FROM bank_accounts WHERE %I = $1 AND balance <> 0', fk.column_name)
                    INTO n USING bad.id;
            ELSE
                EXECUTE format('SELECT count(*) FROM %I WHERE %I = $1', fk.table_name, fk.column_name)
                    INTO n USING bad.id;
            END IF;
            IF n > 0 THEN
                RAISE NOTICE 'player %: % row(s) in %.% -- needs a manual merge into player %',
                    bad.id, n, fk.table_name, fk.column_name, twin;
                blocking := blocking + n;
            END IF;
        END LOOP;

        IF blocking = 0 THEN
            DELETE FROM players WHERE id = bad.id;  -- cascades its empty bank_accounts
            RAISE NOTICE 'player %: empty duplicate of player % deleted', bad.id, twin;
        END IF;
    END LOOP;
END $$;

ALTER TABLE players DROP CONSTRAINT IF EXISTS players_uid_check;
ALTER TABLE players ADD CONSTRAINT players_uid_check CHECK (uid ~ '^[0-9]{17}$');

COMMIT;
