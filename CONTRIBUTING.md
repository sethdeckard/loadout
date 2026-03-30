# Contributing

Loadout is a Go CLI/TUI for managing machine-local Claude and Codex skills from a shared git-backed repo. This guide covers the local contributor workflow and the project constraints worth preserving when you change behavior.

## Prerequisites

- Go installed locally
- `golangci-lint` available for `make lint`
- Git configured for normal commit and push workflows

## Initial Setup

macOS and Linux are the primary supported contributor environments.

Windows compatibility is not currently guaranteed and may drift. If you are developing on Windows, plan on using configurable target paths instead of assuming the default `~/.claude/skills` and `~/.codex/skills` locations fit your environment.

Clone the repo, then enable the repo-local Git hooks:

```bash
git config core.hooksPath .githooks
```

The repo includes a `commit-msg` hook in `.githooks/commit-msg` that enforces the local commit message style:

- subject line required
- subject line starts with a capital letter
- subject line is 50 characters or fewer
- subject line does not end with a period
- body is separated from the subject by a blank line
- body lines wrap at 72 characters

The hook runs with `sh`, so consistent hook enforcement assumes a POSIX shell environment. On Windows, use Git Bash, WSL, or a comparable shell if you want the repo-local hooks to run as documented.

## Build And Run

Use the standard Go commands during development:

```bash
go build ./...
go test ./...
go vet ./...
go run ./cmd/loadout
```

Make targets are available for the common workflows:

```bash
make build
make test
make test-race
make vet
make lint
```

## Development Workflow

1. Make the smallest coherent change you can.
2. Add or update tests with the code change.
3. Run focused tests while iterating.
4. Run the full quality gates before committing.

For most changes, run these before commit:

```bash
make test-race
make vet
make lint
```

If you changed behavior that affects user workflows, architecture, or contributor setup, update the relevant docs in the same change.

## Project Structure

Key directories:

```text
cmd/loadout/            CLI entry point and Cobra commands
internal/app/           Service-layer orchestration
internal/config/        Local config persistence
internal/domain/        Core types, validation, sentinel errors
internal/gitrepo/       Git operations
internal/importer/      Import and candidate discovery
internal/install/       Install/remove logic for target roots
internal/reconcile/     Inventory/status computation
internal/scope/         User vs project scope resolution
internal/skillmd/       SKILL.md parsing/frontmatter helpers
internal/tui/           Bubble Tea presentation layer
testdata/               Fixture trees for tests
```

## Architectural Guardrails

These are the important invariants to preserve:

- The repo stores skill content only, not machine state.
- Install means copy into target roots, not symlink.
- Managed installs are identified by a `.loadout` marker.
- Installed copies are disposable derived artifacts.
- The TUI contains presentation logic only and delegates business actions to `internal/app`.
- `internal/domain` stays dependency-free.

Dependency direction:

```text
cmd -> app, tui
tui -> app, domain
app -> domain, config, registry, gitrepo, install, importer, reconcile, scope
domain -> nothing
```

## Code Style

- Prefer concrete types; introduce interfaces only at the point of use.
- Export only what other packages need.
- Keep package names short and non-stuttering.
- Group imports as stdlib, third-party, then local imports.
- Do not add logging infrastructure; return errors to the caller.

## Error Handling

- Use sentinel errors from `internal/domain/errors.go`.
- Wrap errors with `%w`.
- Check behavior with `errors.Is()`, not string matching.
- Keep error strings lowercase with no trailing punctuation.

## Testing Expectations

- Prefer table-driven tests.
- Name tests as `TestFunc_Scenario`.
- Use `t.TempDir()` for filesystem behavior.
- Use real filesystem state instead of mocks for install/import behavior.
- Mark helpers with `t.Helper()`.
- Use `t.Fatalf` when continuing would make the test meaningless.

Recent regressions have come from install and import edge cases, so changes in those areas should usually include direct regression coverage.

## Commit Messages

Keep commits small and descriptive. The local hook enforces the mechanical parts of the project style, but a good message should still explain the behavior change clearly when the subject alone is not enough.

## Pull Requests

When opening a PR:

- describe the user-visible or architectural change
- call out any risks or edge cases
- mention the tests you ran
- note doc updates when behavior or contributor workflow changed

## Related Docs

- [README.md](README.md) for product usage and CLI/TUI behavior
- [docs/guides.md](docs/guides.md) for workflow-oriented user guides
- [ARCHITECTURE.md](ARCHITECTURE.md) for design details and runtime flows
- [AGENTS.md](AGENTS.md) for codebase-specific implementation rules used by coding agents
