-- Emergency Services Minister: a single top rank above Commissioner in both
-- Police (level 13) and EMS (level 12), with Command and Cabinet, able to
-- promote up to Commissioner. Only adds what's missing. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_es_minister.sql
--
-- Then appoint the minister, e.g.
--   UPDATE players SET cop_level = 13, medic_level = 12 WHERE uid = '<their Steam64>';

BEGIN;
INSERT INTO faction_rank_names (faction, level, name, short_name, slots, promote_up_to, is_command, is_cabinet, description) VALUES
    ('police', 13, 'Emergency Services Minister', 'MIN', 1, 12, true, true, 'Oversees Police and EMS.'),
    ('ems',    12, 'Emergency Services Minister', 'MIN', 1, 11, true, true, 'Oversees Police and EMS.')
ON CONFLICT DO NOTHING;
COMMIT;
