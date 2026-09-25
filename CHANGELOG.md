# Changelog

## [Unreleased]
### Added
- Initial project scaffold: README, directory structure, `.gitignore`, `config.ini.example`.
- Org-standard branding and repo structure: banner (`docs/assets/banner.svg`), `CONTRIBUTING.md`,
  and delivery-board sync workflow (`.github/workflows/sync-to-project.yml`), matching the other
  Tasman Dynamics repos.
- `docs/ROADMAP.md`: phased plan and timeline toward a 2026-12-26 public launch target, tracked
  alongside GitHub issues/milestones on the Delivery Board.
- `docs/ANTI_CHEAT.md`: full threat model and layered defense design (BattlEye, `CfgRemoteExec`
  allowlist, economic integrity, movement/behavior heuristics, graduated response) — expands the
  README's anti-cheat section and Phase 1/3 of the roadmap beyond the original two heuristics.
- `docs/ADMIN_TOOLS.md`: in-game staff menu spec (four permission tiers, gated by a DB-backed
  `staff_ranks` table rather than a hardcoded/client-side flag) and the rank add/remove/assign
  system that backs it — extends Phase 0's schema and Phase 3's admin-tooling scope.
