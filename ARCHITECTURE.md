# Loadout Architecture

## Summary

Loadout is a Go CLI/TUI that manages machine-local installation of agent skills for Claude and Codex from a shared git-backed skills repository.

The architecture is intentionally simple:

- The shared repo stores skill content and metadata only.
- Each machine stores its own configuration locally.
- Installation is copy-based, not symlink-based.
- Installed skill directories are disposable derived artifacts.
- CLI and TUI both delegate business operations to `internal/app.Service`.
- Import preserves authored target declarations by default and does not expand support automatically.

This document describes the current implementation for contributors working in this codebase.

## System Model

Loadout operates across three storage domains:

1. Skill source repository
   A git repository containing a `skills/` tree. Each skill lives in its own directory with `skill.json` metadata and `SKILL.md` content.
2. Local configuration
   Machine-local configuration stored in `~/.config/loadout/config.json`. This defines the source repo path and the target install roots for Claude and Codex.
3. Installed target directories
   Derived copies of skills stored under target roots such as `~/.claude/skills` and `~/.codex/skills`, or under project roots in project mode.

The source repo is the only shared input. Everything else is local machine state.

## Architectural Invariants

- The skills repo contains content only. It does not store machine state.
- Loadout installs by copying directories into target roots.
- Removal deletes installed copies; there are no symlinks or in-place references back to the source repo.
- Installed copies can always be regenerated from the repo.
- The TUI is presentation and interaction state only. It does not implement business rules.
- `internal/domain` remains dependency-free and owns core types, validation, and sentinel errors.

## Layers And Package Responsibilities

Dependency direction:

- `cmd -> app, tui`
- `tui -> app, domain`
- `app -> domain, config, registry, gitrepo, install, importer, reconcile, scope`
- `domain -> nothing`

### Entry Points

- `cmd/loadout/main.go` starts Cobra.
- `cmd/loadout/cmd` contains CLI commands such as `init`, `inventory`, `equip`, `unequip`, `import`, `delete`, `sync`, and `doctor`.
- Running `loadout` with no subcommand starts the Bubble Tea TUI.

### Core Orchestration

`internal/app` is the orchestration boundary for both CLI and TUI.

`app.Service` is the central application API. It loads skills, previews content, equips and unequips targets, syncs the repo, runs health checks, and supports project installs. CLI commands and TUI commands call into the service instead of reaching into lower-level packages directly.

It also owns repo mutation workflows such as importing a local skill directory into the shared repo and discovering unmanaged local skills for import in the TUI.

### Domain Model

`internal/domain` owns pure types and validation:

- `SkillName` identifies a skill.
- `Skill` models repo metadata loaded from `skill.json`.
- `Target` identifies supported install targets such as Claude and Codex.
- Validation ensures skill metadata is structurally valid before the rest of the system acts on it.
- Sentinel errors: `ErrSkillNotFound`, `ErrSkillInstalled`, `ErrInvalidSkill`, `ErrSkillExists`, `ErrImportConflict`, `ErrUnsupportedTarget`, `ErrTargetDisabled`, `ErrRepoNotFound`, `ErrConfigNotFound`.

### Configuration

`internal/config` loads and saves machine-local configuration at `~/.config/loadout/config.json`.

The config currently stores:

- `repo_path`
- per-target `enabled` flags and target roots for Claude and Codex
- `repo_actions.import_auto_commit` and `repo_actions.delete_auto_commit` (default true)

The config is the source of truth for user install roots.

### Registry Loading

`internal/registry` loads the skill catalog from the configured repo path.

Responsibilities:

- scan `skills/`
- require both `skill.json` and `SKILL.md`
- unmarshal metadata into `domain.Skill`
- validate loaded skills
- detect duplicate names
- expose full catalog loading, single-skill lookup, and markdown preview loading

The registry layer treats the git repo as read-only content.

### Import

`internal/importer` owns repo-facing skill import behavior.

Responsibilities:

- inspect an existing local skill directory
- preserve declared `skill.json` targets and target-specific metadata as-authored
- infer minimal metadata when `skill.json` is absent
- strip install-only frontmatter from imported `SKILL.md`
- remove `.loadout` markers from imported repo copies
- discover unmanaged local candidates from enabled target roots
- detect conflicting duplicate local copies across Claude and Codex roots

Import is intentionally conservative. If two local copies disagree on authored metadata or target declarations, Loadout surfaces that as a conflict instead of merging them automatically.

### Skill Markdown

`internal/skillmd` parses YAML frontmatter and headings from `SKILL.md` files. The importer uses this to infer skill name and description when `skill.json` is absent. It extracts the first heading as a candidate name and any frontmatter fields that carry metadata.

### Git Integration

`internal/gitrepo` wraps git operations:

- repo detection
- fast-forward pull
- current HEAD commit lookup
- dirty working tree checks
- repo initialization and clone
- sync-readiness checks for tracked branches

This package is intentionally small and shell-command based.

Import uses a scoped staging helper that commits only the imported skill path when auto-commit is enabled.

### Installation

`internal/install` owns copy/remove behavior in target roots.

Install behavior:

- verify target support
- copy the skill directory into a temp dir on the target filesystem
- transform `SKILL.md` by prepending generated YAML frontmatter
- atomically rename into place
- write a `.loadout` marker file containing repo commit and install timestamp

Remove behavior deletes the installed directory if present.

The `.loadout` marker is how Loadout distinguishes managed installs from unmanaged directories that happen to exist under the same target root.

The generated frontmatter is a derived install artifact. Repo copies of `SKILL.md` remain plain markdown without the prepended frontmatter block.

### Reconciliation

`internal/reconcile` compares desired catalog state with actual installed state.

Today it does not emit imperative operations. Instead, it computes per-skill status used by inventory views and health reporting. It distinguishes:

- installed vs not installed
- managed vs unmanaged
- present in repo vs missing from repo

This is the package that turns filesystem observations into higher-level inventory status flags.

### TUI

`internal/tui` contains Bubble Tea presentation logic:

- screen state
- key handling
- inventory filtering
- details pane rendering
- project/user mode toggles
- settings screen state
- import screen state

All business actions are delegated through `tea.Cmd` helpers that call `app.Service`.

### Project Scope

`internal/scope` resolves whether a command or TUI session is operating in user scope or against a project root.

In project mode, the effective install roots become:

- `<project>/.claude/skills`
- `<project>/.codex/skills`

Project mode is used by CLI commands that accept `--project` and by TUI startup when a compatible project root is detected.

## Key Data Structures And On-Disk Interfaces

### Skill Metadata

`domain.Skill` is loaded from `skill.json` and includes:

- name, description, and tags
- supported targets
- optional target-specific metadata maps for Claude and Codex
- an internal `Path` field pointing back to the skill directory in the repo

`skill.json` is the metadata source of truth. `SKILL.md` is the source markdown body.

### Config File

`config.Config` is persisted at `~/.config/loadout/config.json` and currently contains:

- `repo_path`
- `targets.claude.enabled`
- `targets.claude.path`
- `targets.codex.enabled`
- `targets.codex.path`

This file defines where Loadout reads source content from and where user installs are written.

### Install Marker

Each managed install includes a `.loadout` JSON file with:

- `repo_commit`
- `installed_at`

Current on-disk shape:

```json
{
  "repo_commit": "abc1234",
  "installed_at": "2026-03-21T14:22:33Z"
}
```

This marker supports two important behaviors:

- detect whether an installed skill directory is managed by Loadout
- determine whether a managed install is behind the repo's current HEAD and must be refreshed during sync

## Managed Versus Installed

Loadout makes a deliberate distinction between "installed" and "managed":

- Installed means a skill directory exists under a target root.
- Managed means that installed directory also contains a `.loadout` marker.

This matters because target roots may already contain directories not created by Loadout. The app does not assume every directory is under its control.

`app.Service.scanActual` collects both facts:

- presence of any skill-named directory
- presence of a `.loadout` marker for that directory

`reconcile.Plan` then turns that into inventory flags such as inactive, current, unmanaged, or missing-from-repo.

The import discovery flow depends on the same distinction:

- managed directories contain `.loadout` and are excluded from import candidates
- unmanaged directories under enabled target roots are potential import candidates

## Runtime Flows

### Inventory Flow

User inventory flow:

1. CLI or TUI calls `Service.ListSkills`.
2. The service loads the registry from the configured repo path.
3. The service scans target roots for actual installed directories and `.loadout` markers.
4. `reconcile.Plan` compares registry skills against actual state.
5. The result is returned as `app.SkillView` values for CLI output or TUI rendering.

Project inventory flow:

1. CLI resolves `--project` or TUI switches into detected project mode.
2. `Service.ListSkillsForProject` loads the same registry inventory used by user mode.
3. The service scans both user target roots and the project target roots.
4. The UI renders project install state as the active scope while still showing user installs as informational context.

### Preview Flow

1. CLI inspect or TUI selection requests a preview.
2. `Service.PreviewSkill` loads one skill from the registry.
3. The service reads the source `SKILL.md` from the repo.
4. The preview returns metadata plus raw markdown content.

Preview reads source content, not the transformed installed copy.

### Import Flow

1. CLI `import` or the TUI import screen calls `Service.ImportPath`.
2. The service validates that the configured repo is a git repo.
3. `internal/importer` inspects the source directory and normalizes it into repo format.
4. The importer preserves declared targets from `skill.json` exactly as-authored.
5. If `skill.json` is missing, the importer infers metadata from frontmatter, headings, and explicit or inferred source targets.
6. The importer copies the directory into `repo/skills/<id>/`, strips frontmatter from `SKILL.md`, removes `.loadout`, and writes normalized `skill.json`.
7. If auto-commit is enabled, the service stages only that imported skill path and creates a commit.

### Import Discovery Flow

1. The TUI import screen calls `Service.ListImportCandidates`.
2. The service asks `internal/importer` to scan enabled target roots only.
3. Candidate discovery skips managed directories that already contain `.loadout`.
4. If the same normalized skill name appears in both Claude and Codex roots:
5. identical normalized copies are shown as one duplicate candidate
6. conflicting copies are shown as a blocked candidate with a problem message

Import discovery is a convenience wrapper around import, not a separate business workflow.

### Delete Flow

1. CLI or TUI calls `Service.DeleteSkillEligibility` to check whether deletion is safe.
2. The service verifies the skill exists in the registry and checks for managed installs that would block deletion.
3. If eligible, `Service.DeleteSkill` removes the skill directory from the repo's `skills/` tree.
4. If `repo_actions.delete_auto_commit` is enabled, the service stages the removal and creates a commit.

Deletion is blocked when managed installs (user or project) still reference the skill. Unmanaged copies do not block deletion.

### Equip And Unequip Flow

User equip:

1. CLI or TUI calls `EnableSkillTarget` or `ToggleSkillTarget`.
2. The service loads the requested skill from the registry.
3. The service verifies the skill supports the requested target.
4. The service resolves the configured target root.
5. `install.Install` copies the repo content, transforms `SKILL.md`, and writes a `.loadout` marker.

User unequip:

1. CLI or TUI calls `DisableSkillTarget` or `ToggleSkillTarget`.
2. The service resolves the configured target root.
3. `install.Remove` deletes the installed directory if present.

Project-local equip and unequip follow the same install/remove path, but target roots are derived from the project root instead of user config.

### Sync Flow

1. CLI or TUI calls `Service.SyncRepoWithResult`.
2. The service verifies the configured repo is a git repo.
3. Best-effort startup checks may already have highlighted sync attention by combining:
   repo dirtiness,
   remote-behind status from `git fetch`,
   and managed-install drift from `.loadout.repo_commit` versus local HEAD.
4. `gitrepo.Pull` performs `git pull --ff-only`.
5. The service reloads the registry and resolves the new local repo HEAD.
6. For each relevant target root, `install.ScanManaged` finds managed installs.
   Relevant means user roots always, plus the active project roots when sync runs in project scope.
7. Each managed install's `.loadout.repo_commit` is compared to the current repo HEAD.
8. Managed installs still present in the registry, still supporting that target, and behind HEAD are reinstalled in place from repo content.

Sync is now the explicit apply step for managed-install freshness. It refreshes outdated managed copies automatically after the repo is updated, while leaving unmanaged directories untouched.

### Doctor Flow

1. CLI or TUI calls `Service.Doctor`.
2. The service checks whether the configured repo exists and looks like a git repo.
3. The service checks sync readiness for the current tracked branch.
4. The service attempts to load the registry.
5. The service reports configured target roots.
6. If the registry loaded successfully, the service scans managed installs and reports whether any are missing from the registry.

Doctor is a read-only health report. It does not attempt repair.

### TUI Flow

1. Root command creates `app.Service` from local config.
2. The root command detects a project root from CWD and passes it to the TUI. When a project is detected the TUI starts in project scope; otherwise it starts in user scope. The `--user` flag skips project detection and forces user scope.
3. The TUI model loads skills on startup for the active scope.
4. Key actions map to `tea.Cmd` helpers in `internal/tui/commands.go`.
5. Those commands call into `app.Service`.
6. Result messages update the model and refresh the current scope view as needed.

This keeps business rules out of update/render code and makes the TUI a thin stateful shell around the service layer.

## User Versus Project-Local Mode

Loadout supports two installation scopes:

- User mode writes to paths from `config.Config`.
- Project mode writes under a detected project root.

Project mode is resolved through `internal/scope`:

- `Resolve` handles `--project`
- `DetectProjectRoot` walks upward to a git root and requires either `.claude/` or `.codex/`

This allows a contributor to manage repository-specific agent skill directories without changing user configuration.

The TUI defaults to project mode at startup when a compatible project is detected, and shows a one-time status cue confirming the active scope. The `tab` key toggles between scopes during a session.

In the TUI, project mode reuses the full registry-backed inventory rather than switching to an installed-only project list. The selected scope changes:

- which install roots `c`, `x`, and `a` operate on
- which install status is rendered as primary
- which utility hints are shown, including project import hints

User installs remain visible in project mode as read-only context so the user can choose to add a project copy without first removing the user install.

## Testing Strategy

The codebase tests architecture-critical behavior using stdlib-only, table-driven tests.

Notable coverage areas:

- metadata validation in `internal/domain`
- registry loading and invalid fixture handling
- filesystem copy/install behavior with `t.TempDir()`
- reconcile status computation
- service-level orchestration including doctor, sync error paths, and project flows
- command-level initialization flow

Filesystem behavior is tested against real temp directories rather than mocks. That matches the app's design, where filesystem state is part of the core behavior.

## Safe Extension Guidelines

When changing the system:

- Add or change business workflows in `internal/app` first, then expose them through CLI/TUI.
- Keep UI concerns in `internal/tui`; do not move install, registry, or policy logic there.
- Extend `domain.Skill` only when the new field belongs in repo metadata.
- Preserve marker-based managed install detection unless intentionally redesigning ownership semantics.
- Preserve copy-based installation and target-specific `SKILL.md` transformation behavior.
- Prefer concrete types and narrow package APIs over broad shared interfaces.

Good extension points include:

- adding a new CLI command that calls into `app.Service`
- expanding doctor or inventory reporting
- supporting additional target-specific metadata fields
- adding more project-mode workflows

Changes that should be treated carefully:

- altering target root semantics
- changing `.loadout` marker format
- moving business logic into TUI update/view code
- making the repo responsible for local machine state

## Current Limitations

The current architecture deliberately keeps state light.

- There is no separate persistent machine-state database beyond config and install markers.
- Reconcile computes status, not executable convergence operations.
- Sync refreshes managed installs only; it does not attempt to reconcile unmanaged directories.
- Preview reads source markdown, not the installed transformed copy.

These are current implementation facts, not accidental omissions.
