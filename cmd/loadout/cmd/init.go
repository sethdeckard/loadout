package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/gitrepo"
)

var (
	initRepoPath     string
	initCloneURL     string
	initTargets      string
	initClaudeSkills string
	initCodexSkills  string
)

func init() {
	initCmd.Flags().StringVar(&initRepoPath, "repo", "", "path to skills git repository")
	initCmd.Flags().StringVar(&initCloneURL, "clone", "", "clone skills repo from URL")
	initCmd.Flags().StringVar(&initTargets, "targets", "", "comma-separated enabled targets (claude, codex)")
	initCmd.Flags().StringVar(&initClaudeSkills, "claude-skills", "", "Claude skills install path")
	initCmd.Flags().StringVar(&initCodexSkills, "codex-skills", "", "Codex skills install path")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Initialize loadout configuration",
	GroupID: "setup",
	Long: `Interactive wizard to set up loadout.

Without flags, init presents three choices:
  1) Create a new empty skills repo
  2) Clone an existing skills repo from a URL
  3) Use an existing local skills repo

For scriptable setup, use --clone or --repo with --targets.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	return runInitWith(cmd.OutOrStdout(), os.Stdin)
}

func runInitWith(out io.Writer, in io.Reader) error {
	scanner := bufio.NewScanner(in)
	cfg := config.Default()

	// Resolve repo source
	var repoPath, cloneURL string

	switch {
	case initRepoPath == "" && initCloneURL == "":
		choice := promptChoice(scanner, out, "Skills repo", []string{
			"Create a new empty repo",
			"Clone from a URL",
			"Use an existing local path",
		}, 0)
		switch choice {
		case 0:
			repoPath = defaultRepoPath()
		case 1:
			fmt.Fprintf(out, "Clone URL: ")
			if scanner.Scan() {
				cloneURL = strings.TrimSpace(scanner.Text())
			}
			if cloneURL == "" {
				return fmt.Errorf("clone URL is required")
			}
			repoPath = promptCloneDestination(scanner, out, defaultRepoPath())
		case 2:
			fmt.Fprintf(out, "Repo path: ")
			if scanner.Scan() {
				repoPath = strings.TrimSpace(scanner.Text())
			}
			if repoPath == "" {
				return fmt.Errorf("repo path is required")
			}
		}
	case initCloneURL != "":
		cloneURL = initCloneURL
		repoPath = initRepoPath
	default:
		repoPath = initRepoPath
	}

	if repoPath == "" {
		repoPath = defaultRepoPath()
	}
	repoPath = expandHome(repoPath)

	// Handle clone
	if cloneURL != "" {
		if dirNonEmpty(repoPath) {
			return fmt.Errorf("clone destination already exists: %s; use --repo to adopt an existing path", repoPath)
		}
		if err := gitrepo.Clone(cloneURL, repoPath); err != nil {
			return fmt.Errorf("clone repo: %w", err)
		}
		fmt.Fprintf(out, "Cloned %s to %s\n", cloneURL, repoPath)
	}

	// Setup repo based on current state
	if err := setupRepo(out, repoPath, cloneURL != ""); err != nil {
		return err
	}
	cfg.RepoPath = repoPath

	claudeEnabled := cfg.Targets.Claude.Enabled
	codexEnabled := cfg.Targets.Codex.Enabled
	if initTargets == "" {
		defaultTargets := "claude,codex"
		initTargets = promptWithDefault(scanner, out, "Enabled targets (claude, codex, claude,codex)", defaultTargets)
	}
	if initTargets != "" {
		parsedClaude, parsedCodex, err := parseEnabledTargets(initTargets)
		if err != nil {
			return err
		}
		claudeEnabled = parsedClaude
		codexEnabled = parsedCodex
	}
	cfg.Targets.Claude.Enabled = claudeEnabled
	cfg.Targets.Codex.Enabled = codexEnabled

	// Resolve target paths
	claudePath := initClaudeSkills
	if claudePath == "" && claudeEnabled {
		claudePath = promptWithDefault(scanner, out, "Claude skills path", cfg.Targets.Claude.Path)
	}
	if claudePath != "" {
		cfg.Targets.Claude.Path = expandHome(claudePath)
	}

	codexPath := initCodexSkills
	if codexPath == "" && codexEnabled {
		codexPath = promptWithDefault(scanner, out, "Codex skills path", cfg.Targets.Codex.Path)
	}
	if codexPath != "" {
		cfg.Targets.Codex.Path = expandHome(codexPath)
	}

	if cfg.Targets.Claude.Enabled && cfg.Targets.Claude.Path == "" {
		return fmt.Errorf("claude skills path is required when claude is enabled")
	}
	if cfg.Targets.Codex.Enabled && cfg.Targets.Codex.Path == "" {
		return fmt.Errorf("codex skills path is required when codex is enabled")
	}

	// Save config
	cfgPath := config.DefaultPath()
	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(out, "Config written to %s\n", cfgPath)

	// Ensure target directories
	for _, target := range domain.AllTargets() {
		root := cfg.Targets.Path(target)
		if root != "" {
			if err := fsx.EnsureDir(root); err != nil {
				return fmt.Errorf("ensure target dir: %w", err)
			}
		}
	}

	fmt.Fprintln(out, "Loadout initialized successfully.")
	return nil
}

// setupRepo handles the branching logic for repo state.
// cloned indicates the repo was just cloned — bootstrap commits are skipped
// for clones so an unborn HEAD can receive the first remote commit via pull.
func setupRepo(out io.Writer, repoPath string, cloned bool) error {
	switch {
	case !pathExists(repoPath):
		if err := gitrepo.Init(repoPath); err != nil {
			return fmt.Errorf("init repo: %w", err)
		}
		if err := scaffoldEmptySkillsDir(repoPath); err != nil {
			return fmt.Errorf("scaffold: %w", err)
		}
		if err := writeRepoReadme(repoPath); err != nil {
			return fmt.Errorf("write readme: %w", err)
		}
		fmt.Fprintf(out, "Created new skills repo at %s\n", repoPath)
	case !gitrepo.IsRepo(repoPath):
		return fmt.Errorf("path %q exists but is not a git repository", repoPath)
	case fsx.DirExists(filepath.Join(repoPath, "skills")):
		fmt.Fprintf(out, "Using existing skills repo at %s\n", repoPath)
	default:
		if err := scaffoldEmptySkillsDir(repoPath); err != nil {
			return fmt.Errorf("scaffold: %w", err)
		}
		if err := writeRepoReadme(repoPath); err != nil {
			return fmt.Errorf("write readme: %w", err)
		}
		fmt.Fprintf(out, "Scaffolded skills/ in %s\n", repoPath)
	}

	// Bootstrap commit for locally-created/adopted repos only.
	// Clones keep unborn HEAD so git pull can bring the first remote commit.
	if !cloned {
		if _, err := gitrepo.HeadCommit(repoPath); err != nil {
			var paths []string
			for _, p := range []string{"skills", "README.md"} {
				if pathExists(filepath.Join(repoPath, p)) {
					paths = append(paths, p)
				}
			}
			if len(paths) > 0 {
				if err := gitrepo.AddPathsAndCommit(repoPath, paths, "Initial skills"); err != nil {
					return fmt.Errorf("initial commit: %w", err)
				}
			}
		}
	}
	return nil
}

// scaffoldEmptySkillsDir creates an empty skills/ directory with a .gitkeep.
func scaffoldEmptySkillsDir(repoPath string) error {
	skillsDir := filepath.Join(repoPath, "skills")
	if err := fsx.EnsureDir(skillsDir); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillsDir, ".gitkeep"), []byte{}, 0o644)
}

func writeRepoReadme(repoPath string) error {
	path := filepath.Join(repoPath, "README.md")
	if pathExists(path) {
		return nil
	}
	readme := `# Skills Repository

This repository contains skills for [Loadout](https://github.com/sethdeckard/loadout),
a manager for Claude and Codex agent skills.

## Structure

` + "```" + `
skills/
  my-skill/
    skill.json    # Metadata: name, description, tags, targets
    SKILL.md      # Skill content (markdown, no frontmatter)
` + "```" + `

## skill.json Format

` + "```json" + `
{
  "name": "my-skill",
  "description": "What this skill does.",
  "tags": ["topic"],
  "targets": ["claude", "codex"],
  "claude": {
    "allowed-tools": "Read, Grep"
  },
  "codex": {}
}
` + "```" + `

- **name**: Lowercase, hyphenated identifier (must match directory name under ` + "`skills/`" + `).
- **targets**: Which agents can use this skill ("claude", "codex", or both).
- **claude/codex**: Optional per-target frontmatter fields. These are merged into
  the YAML frontmatter when loadout installs the skill.

## SKILL.md

Write the skill body as plain markdown. Loadout generates target-specific YAML
frontmatter (name, description, plus any fields from the target config) when
installing to each target directory.

## Adding a Skill

The fastest way to add a skill is to import an existing one:

` + "```bash" + `
loadout import path/to/my-skill
` + "```" + `

To create a skill from scratch:

1. Create a new directory under skills/ (e.g. skills/my-skill/).
2. Add skill.json with the required fields, making sure ` + "`name`" + ` matches the directory name.
3. Add SKILL.md with the skill instructions.
4. Commit and push.
5. Run ` + "`loadout sync`" + ` to refresh, then ` + "`loadout equip my-skill`" + `.
`
	return os.WriteFile(path, []byte(readme), 0o644)
}

func promptWithDefault(scanner *bufio.Scanner, out io.Writer, prompt, defaultVal string) string {
	fmt.Fprintf(out, "%s [%s]: ", prompt, defaultVal)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			return val
		}
	}
	return defaultVal
}

func promptCloneDestination(scanner *bufio.Scanner, out io.Writer, defaultPath string) string {
	path := promptWithDefault(scanner, out, "Clone destination", defaultPath)
	for dirNonEmpty(expandHome(path)) {
		fmt.Fprintf(out, "path already exists and is not empty: %s; choose a different clone destination or use an existing local path\n", expandHome(path))
		prev := path
		path = promptWithDefault(scanner, out, "Clone destination", path)
		if path == prev {
			// Input ended (EOF) or user accepted same default — can't make progress
			break
		}
	}
	return path
}

func promptChoice(scanner *bufio.Scanner, out io.Writer, prompt string, options []string, defaultIdx int) int {
	fmt.Fprintf(out, "%s:\n", prompt)
	for i, opt := range options {
		marker := "  "
		if i == defaultIdx {
			marker = "> "
		}
		fmt.Fprintf(out, "  %s%d) %s\n", marker, i+1, opt)
	}
	fmt.Fprintf(out, "Choice [%d]: ", defaultIdx+1)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			if n, err := strconv.Atoi(val); err == nil && n >= 1 && n <= len(options) {
				return n - 1
			}
		}
	}
	return defaultIdx
}

func parseEnabledTargets(raw string) (bool, bool, error) {
	var claudeEnabled bool
	var codexEnabled bool
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "":
			continue
		case "claude":
			claudeEnabled = true
		case "codex":
			codexEnabled = true
		default:
			return false, false, fmt.Errorf("unsupported target %q", part)
		}
	}
	return claudeEnabled, codexEnabled, nil
}

func defaultRepoPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "loadout", "repo")
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirNonEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
