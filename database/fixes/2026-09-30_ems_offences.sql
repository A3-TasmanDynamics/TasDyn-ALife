-- EMS offences: EMS wording for the offences copied from Police, and
-- EMS-only offences. Renames only rows still at their seeded name. Safe to
-- re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_ems_offences.sql

BEGIN;
UPDATE faction_offences o SET name = r.new_name
FROM (VALUES
    ('Wrong weapon, equipment or clothing equipped', 'Wrong equipment or clothing equipped'),
    ('Using weapons, equipment or vehicles without the training', 'Using equipment or vehicles without the training'),
    ('Committing crimes as a civilian under an officer''s name', 'Committing crimes as a civilian under a medic''s name'),
    ('Deliberate murder of an officer', 'Deliberate murder of emergency services personnel'),
    ('Selling faction weapons or equipment', 'Selling faction equipment or medical supplies')
) AS r(old_name, new_name)
WHERE o.faction = 'ems' AND o.name = r.old_name
  AND NOT EXISTS (SELECT 1 FROM faction_offences x WHERE x.faction = 'ems' AND x.name = r.new_name);

INSERT INTO faction_offences (faction, tier, name, min_points, max_points, sort) VALUES
    ('ems', 'low',  'Failure to respond to an emergency call',        0, 10, 25),
    ('ems', 'low',  'Leaving a patient without a handover',           5, 15, 26),
    ('ems', 'high', 'Refusing treatment to a patient without reason', 10, 20, 27)
ON CONFLICT DO NOTHING;
COMMIT;
