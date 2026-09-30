-- EMS (Medical) setup: an ambulance-style rank ladder, Air Ambulance and
-- Education & Training divisions, and EMS qualifications. Only adds what's
-- missing, so edits made in the command panel are kept. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_ems_setup.sql
--
-- Then give the EMS lead their rank (Commissioner is level 11), e.g.
--   UPDATE players SET medic_level = 11 WHERE uid = '<their Steam64>';

BEGIN;
INSERT INTO faction_rank_names (faction, level, name, short_name, promote_up_to, is_command, is_cabinet) VALUES
    ('ems',  1, 'Trainee Paramedic',        'TP',   0, false, false),
    ('ems',  2, 'Paramedic',                'PM',   0, false, false),
    ('ems',  3, 'Senior Paramedic',         'SP',   0, false, false),
    ('ems',  4, 'Intensive Care Paramedic', 'ICP',  2, false, false),
    ('ems',  5, 'Clinical Team Leader',     'CTL',  3, false, false),
    ('ems',  6, 'Station Officer',          'SO',   4, false, false),
    ('ems',  7, 'Duty Operations Manager',  'DOM',  5, true,  false),
    ('ems',  8, 'Area Manager',             'AM',   6, true,  false),
    ('ems',  9, 'Assistant Commissioner',   'AC',   7, true,  true),
    ('ems', 10, 'Deputy Commissioner',      'DC',   9, true,  true),
    ('ems', 11, 'Commissioner',             'COMM', 10, true, true)
ON CONFLICT DO NOTHING;

INSERT INTO faction_quals (faction, key, name, body, sort) VALUES
    ('ems', 'BLS', 'Basic Life Support',        'CPR, revive, bleeding control and scene safety. Required for everyone.', 1),
    ('ems', 'ALS', 'Advanced Life Support',     'Advanced airway, drugs and trauma care.', 2),
    ('ems', 'EVD', 'Emergency Vehicle Driving', 'Driving under lights and sirens.', 3),
    ('ems', 'FTO', 'Field Training Officer',    'Can train and evaluate trainee paramedics.', 4),
    ('ems', 'AIR', 'Flight Paramedic',          'Air Ambulance flight and winch certification.', 5),
    ('ems', 'EDU', 'Clinical Educator',         'Can run Education & Training courses.', 6)
ON CONFLICT DO NOTHING;

INSERT INTO faction_divisions (faction, key, name, color, roles, required_qual, min_level, sort) VALUES
    ('ems', 'AIR', 'Air Ambulance',        '#c4b5fd', ARRAY['Commander', 'Second in Command', 'Flight Trainer', 'Senior Flight Paramedic', 'Flight Paramedic', 'Trial Flight Paramedic'], 'AIR', 0, 1),
    ('ems', 'EDU', 'Education & Training', '#86efac', ARRAY['Commander', 'Second in Command', 'Senior Educator', 'Educator', 'Trainee Educator'], NULL, 0, 2)
ON CONFLICT DO NOTHING;
COMMIT;
