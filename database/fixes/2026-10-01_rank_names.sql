-- Staff rank names: level 100 (head_admin) is now "Management" and the
-- level 99 rank (founder) "Development". Only display names change; the
-- keys, levels, permissions and Discord role mappings stay as they are.
-- Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-01_rank_names.sql

BEGIN;
UPDATE staff_ranks SET display_name = 'Management' WHERE key = 'head_admin';
UPDATE staff_ranks SET display_name = 'Development' WHERE key = 'founder';
COMMIT;
