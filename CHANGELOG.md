# Changelog

## [0.2.0] - 2026-03-29

Improved TUI workflows for unmanaged skills, navigation, and first-run guidance.

### Features

- Unmanaged skills now appear in inventory as `not in repo`, including ready project-local skills discovered in project scope
- Skill details can preview local unmanaged and project-local import sources before import
- Repo-only actions stay blocked for `not in repo` skills, with clearer import guidance in the details pane
- First-run empty state in the details pane now guides users when no managed or unmanaged skills are found in the current scope
- Added vim-style half-page navigation with `ctrl+u` and `ctrl+d` across inventory, import, settings, and help views
- Improved long-list browsing and action highlighting in inventory and import views
- Added workflow-oriented guides for importing skills and authoring new skills with `loadout-smith`

## [0.1.0] - 2026-03-28

Initial release.

### Features

- TUI and CLI for managing Claude and Codex skills from a shared git repo
- Multi-target support for Claude, Codex, or both
- User and project install scopes
- Import of existing local skills into the shared repo, with optional auto-commit
- Skill deletion with safety checks and confirmation
- Repo sync with push, fast-forward pull, and managed install refresh
- Doctor command for configuration and install health checks
- Settings screen for repo actions, target paths, and target toggles
