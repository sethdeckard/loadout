# Loadout - Agent Instructions

## Overview

Loadout is a Go TUI/CLI app that manages machine-local installation of skills for Claude and Codex from a shared private git repo.

## Skill Format References

- OpenAI open format: <https://agentskills.io/specification>
- Anthropic Claude skills: <https://code.claude.com/docs/en/skills#extend-claude-with-skills>

## Build & Test

```bash
go build ./...          # Compile all packages
go test ./...           # Run all tests
go vet ./...            # Static analysis
go run ./cmd/loadout    # Run the app (launches TUI if configured)
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the `loadout` binary |
| `make test` | Run all tests |
| `make test-race` | Run all tests with race detector |
| `make cover` | Generate coverage report (text) |
| `make cover-html` | Generate coverage report (HTML) |
| `make vet` | Run `go vet` |
| `make lint` | Run `golangci-lint` |
| `make clean` | Remove binary and coverage files |

## Project Structure

```
cmd/loadout/            Entry point
cmd/loadout/cmd/        Cobra subcommands (root, init, equip, unequip, etc.)
internal/domain/        Pure types: Target, SkillName, Skill, errors, validation
internal/fsx/           Filesystem helpers: CopyDir, WriteJSONAtomic, EnsureDir, DirExists, Exists, ListDirs
internal/config/        Config persistence (~/.config/loadout/config.json)
internal/registry/      Loads skill definitions from git repo's skills/ directory
internal/gitrepo/       Git operations: IsRepo, Init, Clone, Pull, Push, HeadCommit, AddPathsAndCommit, sync checks
internal/install/       Install/Remove skills to/from target directories
internal/importer/      Import local skills into the shared repo
internal/skillmd/       Parse YAML frontmatter and headings from SKILL.md files
internal/scope/         Resolve user vs project-local install roots
internal/reconcile/     Compute per-skill inventory status from repo + filesystem
internal/app/           Service layer orchestrating all packages
internal/tui/           Bubble Tea TUI (presentation only, no business logic)
testdata/               Registry fixtures for tests
```

## Architecture Rules

- **Dependency direction**: cmd -> app, tui; tui -> app, domain; app -> everything; domain -> nothing
- **TUI contains no business logic** — it calls app.Service for all operations
- **No symlinks** — install means copy, remove means delete
- **Repo contains content only** — no machine state in the git repo
- **State is local** — each machine has its own config.json and .loadout marker files
- **Installed copies are disposable** — derived from repo, can be regenerated

## Code Style

- **Constructors**: `New()` returns concrete types, not interfaces
- **Minimal exports**: only export what other packages need
- **Concrete types by default** — define interfaces only at the point of use
- **Package names**: short, no stutter (e.g. `fsx`, not `fsxutils`)
- **No logging library** — errors propagate up, CLI prints to stdout/stderr
- **Import groups**: stdlib, then third-party, then local (`github.com/sethdeckard/loadout/...`), separated by blank lines

## Error Handling

- Sentinel errors live in `domain/errors.go` with `Err` prefix
- Current sentinels: `ErrSkillNotFound`, `ErrSkillInstalled`, `ErrInvalidSkill`, `ErrSkillExists`, `ErrImportConflict`, `ErrUnsupportedTarget`, `ErrTargetDisabled`, `ErrRepoNotFound`, `ErrConfigNotFound`
- Error strings: lowercase, no trailing punctuation
- Always wrap with `%w` (never `%v` for errors)
- Check with `errors.Is()` — never match error strings
- No logging library — errors propagate to the caller

## Testing Patterns

- **Table-driven tests** with `tt` loop variable
- **Naming**: `TestFunc_Scenario` (e.g. `TestInstall_UnsupportedTarget`)
- **Filesystem tests**: use `t.TempDir()`, never mock the filesystem
- **Test helpers**: mark with `t.Helper()`, fatal on setup errors
- **Error checking**: use `errors.Is()`, never match error strings
- **Stdlib only** — no testify, no mock libraries
- **Fixtures**: stored in `testdata/` directories
- **`t.Fatalf`** when continuing is meaningless, `t.Errorf` when other checks still add value

## After Changes

After making code changes, run:

```bash
make test              # Run all tests
make lint              # Run golangci-lint (includes formatting checks)
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `loadout` | Opens TUI |
| `loadout init` | Interactive setup (create, clone, or use existing repo) |
| `loadout inventory` | List skills + status |
| `loadout inspect <name>` | Preview skill details |
| `loadout equip <name> --target claude` | Enable + install |
| `loadout unequip <name> --target codex` | Disable + remove |
| `loadout import <path>` | Import local skill into repo |
| `loadout delete <name>` | Delete skill from repo |
| `loadout sync` | Sync repo and refresh outdated managed installs |
| `loadout doctor` | Health check |

## Before Committing

Run all quality gates — CI will enforce these:

```bash
make test-race          # Tests with race detector
make vet                # Static analysis
make lint               # golangci-lint
```

All three must pass before committing.
