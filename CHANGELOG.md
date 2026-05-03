# Changelog

## [0.4.0] - 2026-05-03

### Features

- `loadout share <name>` packages a skill into a portable `.tar.gz` archive containing `claude-build/`, `codex-build/`, `loadout-source/`, and a `README.md` with install instructions for all three flows
- Share archives omit `*-build/` subdirectories for targets the skill does not declare, and strip `.loadout` markers and OS junk (`.DS_Store`, `Thumbs.db`, `.git/`)
- `--out` accepts a directory (writes `<name>.tar.gz` inside) or a full file path; existing output files are left untouched and the command errors instead of overwriting

### Changes

- Configuration now lives at `~/.config/loadout/config.toml` (TOML instead of JSON). Existing `config.json` files are read once and converted to TOML on first load; the original JSON file is left in place as a backup.

## [0.3.0] - 2026-04-03

### Features

- Scroll hints show `ctrl+u/d` and `g/G` inline with scroll indicators in the details and import preview panes
- Import preview pane is now scrollable with `h/l` focus switching and focus-aware border styling

### Bug Fixes

- YAML frontmatter values from skill.json are now double-quoted, fixing invalid YAML when descriptions contain colons or other special characters
- Source SKILL.md frontmatter is stripped before prepending generated metadata, preventing duplicate frontmatter blocks in installed files

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
- Increase operational safety with unmanaged directories and symlink dereferencing.

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
