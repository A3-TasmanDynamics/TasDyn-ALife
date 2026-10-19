-- In-game admin permissions (the phone's Admin app): adds the "In game"
-- catalogue keys (src/website/internal/auth/permissions.go) to the built-in
-- ranks by level, the same way schema.sql seeds a fresh install. Safe to
-- run more than once; ranks edited on /admin/roles keep their other keys,
-- and a rank only gains keys here (nothing is taken away).

INSERT INTO rank_permissions (rank_id, command_key)
SELECT sr.id, k.command_key
FROM staff_ranks sr
JOIN (VALUES
    ('game.admin', 10),
    ('game.spectate', 10),
    ('game.teleport', 20),
    ('game.heal', 20),
    ('game.freeze', 20),
    ('game.kick', 20),
    ('game.warn', 20),
    ('game.markers', 20),
    ('game.vehicles', 20),
    ('game.self', 40),
    ('game.spawn', 100)
) AS k(command_key, seed_level) ON sr.level >= k.seed_level
WHERE sr.key IN ('trial_mod', 'moderator', 'admin', 'head_admin')
ON CONFLICT DO NOTHING;
