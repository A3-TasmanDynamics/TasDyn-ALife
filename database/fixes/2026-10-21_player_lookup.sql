-- Player Lookup additions (website): staff notes about players, and the
-- permissions for its new actions. Safe to run more than once; ranks edited
-- on /admin/roles keep their other keys, and a rank only gains keys here.

-- staff_notes.kind gains 'player' (notes about a player, not a staff member).
ALTER TABLE staff_notes DROP CONSTRAINT IF EXISTS staff_notes_kind_check;
ALTER TABLE staff_notes ADD CONSTRAINT staff_notes_kind_check CHECK (kind IN ('note', 'promotion', 'player'));

INSERT INTO rank_permissions (rank_id, command_key)
SELECT sr.id, k.command_key
FROM staff_ranks sr
JOIN (VALUES
    ('players.notes', 10),
    ('players.licences', 40),
    ('players.vehicle_actions', 40)
) AS k(command_key, seed_level) ON sr.level >= k.seed_level
WHERE sr.key IN ('trial_mod', 'moderator', 'admin', 'head_admin')
ON CONFLICT DO NOTHING;
