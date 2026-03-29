# Loadout Guides

Practical workflows for getting skills into Loadout and keeping them managed from a repo you control.

## Guide 1: Import Existing Claude/Codex Skills

Use this when you already have skills living outside your Loadout-managed repo and want to bring them under repo management.

### What import does

- Copies an existing local skill directory into your repo under `skills/<name>/`
- Preserves declared metadata from `skill.json` when present
- Infers metadata from `SKILL.md` and the source root when `skill.json` is missing
- Refuses imports when the same skill appears with conflicting metadata across Claude and Codex roots

After import, the repo copy becomes the source of truth. Edit the skill there, not in the installed copy.

### Fastest path: TUI bulk import

1. Run `loadout`
2. Press `i` to open import
3. Review the discovered unmanaged local skills
4. Import one candidate or import all ready candidates

This is the best path when you are migrating an existing local skill collection.

### Single skill path: CLI import

```bash
loadout import /path/to/local-skill
```

Use this when you already know the directory you want to import.

### Resulting repo shape

Imported skills land in:

```text
skills/<name>/
  skill.json
  SKILL.md
  references/...
  scripts/...
```

Supporting files are preserved. Loadout strips install-time frontmatter from the repo copy of `SKILL.md` and removes any `.loadout` marker.

## Guide 2: Author A New Skill In-Repo With `loadout-smith`

Use this when you want an agent to create a brand-new Loadout-compatible skill directly inside your managed repo.

### What `loadout-smith` is

`loadout-smith` is a skill that teaches an agent how to create or update a Loadout-compatible skill under:

```text
skills/<name>/
```

It lives in the example repo here:

- [loadout-example-skills](https://github.com/sethdeckard/loadout-example-skills)
- specifically [`skills/loadout-smith/`](https://github.com/sethdeckard/loadout-example-skills/tree/main/skills/loadout-smith)

You usually do not want to adopt that example repo as your real skills repo. It is mainly a source of structural examples; most of the skills are intentionally playful, with `loadout-smith` and a small number of others being the practical exceptions.

### Easiest way to get it into your managed repo

Copy this directory into your own Loadout repo:

```text
skills/loadout-smith/
```

That gives your agent an in-repo authoring skill immediately.

### Alternative: clone the example repo, then import to user scope

This is useful when you want `loadout-smith` available as a personal skill in user scope without making the example repo your source-of-truth repo.

1. Clone [loadout-example-skills](https://github.com/sethdeckard/loadout-example-skills)
2. Open `loadout`
3. Stay in user scope, or switch to it with `tab`
4. Press `i` to open import
5. Use the file browser to scan the cloned example repo and import `loadout-smith`
6. Equip it in user scope

That gives you a user-installed `loadout-smith` skill you can use while continuing to keep your own managed repo separate.

### What the agent should create

Minimum skill layout:

```text
skills/<name>/
  skill.json
  SKILL.md
```

Optional support files:

```text
skills/<name>/
  references/
  scripts/
  templates/
```

Authoring rules to keep:

- use a kebab-case skill name
- keep `skill.json.name` equal to the directory name
- set `targets` explicitly
- default to both `claude` and `codex` unless the skill is genuinely target-specific
- use target-specific metadata only when one target needs different installed frontmatter

## Guide 3: Choose The Right Creation Path

There are three good ways to create a managed skill.

### Path A: Author directly in the managed repo

Use this when:

- the skill is new
- you already know it belongs in your shared skills repo
- you want the cleanest long-term workflow

Best tool:

- `loadout-smith`

### Path B: Have an agent create it elsewhere, then import immediately

Use this when:

- you want the agent to draft the skill outside your repo first
- you want a quick review pass before it becomes managed
- you still want Loadout to normalize it into your repo right away

Best tool:

- your normal agent workflow, followed by `loadout import <path>` or TUI import

### Path C: Migrate an existing unmanaged skill

Use this when:

- the skill already lives in `~/.claude/skills`, `~/.codex/skills`, or a project-local skills directory
- you are bringing an existing library under Loadout management

Best tool:

- TUI import for bulk migration

### Recommended steady state

Create however is easiest, but get the skill into your managed repo early. After that:

1. edit the repo copy
2. equip through Loadout
3. sync from the repo as the source of truth

That keeps installed copies disposable and avoids drift between machines.
