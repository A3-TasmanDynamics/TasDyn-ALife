# Changelog

## [Unreleased]
### Added
- Initial project scaffold: README, directory structure, `.gitignore`, `config.ini.example`.
- Org-standard branding and repo structure: banner (`docs/assets/banner.svg`), `CONTRIBUTING.md`,
  and delivery-board sync workflow (`.github/workflows/sync-to-project.yml`), matching the other
  Tasman Dynamics repos.
- `docs/ROADMAP.md`: phased plan and timeline toward a public launch target, tracked alongside
  GitHub issues/milestones on the Delivery Board.
- `docs/ANTI_CHEAT.md`: full threat model and layered defense design (BattlEye, `CfgRemoteExec`
  allowlist, economic integrity, movement/behavior heuristics, graduated response) — expands the
  README's anti-cheat section and Phase 1/3 of the roadmap beyond the original two heuristics.
- `docs/ADMIN_TOOLS.md`: in-game staff menu spec, expanded to feature parity with infiSTAR and
  Fini Anti-Hack & Admin Tools plus arsenal editing (per-faction item pools, loadout presets),
  backed by a DB-driven `staff_ranks` table + per-command permission overrides, a debug console,
  and in-menu logging/metrics — extends Phase 0's schema and Phase 3's scope substantially.
- Roadmap revised: public launch target moved from 2026-12-26 to **2027-01-20** (~4 added weeks,
  entirely inside Phase 3's window) once the admin-tooling scope above was made launch-critical
  rather than deferred — flagged explicitly in ROADMAP.md as an estimate pending confirmation.
