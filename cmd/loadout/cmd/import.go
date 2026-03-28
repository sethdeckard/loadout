package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/domain"
)

var (
	importTargets string
	importCommit  bool
)

func init() {
	importCmd.Flags().StringVar(&importTargets, "targets", "", "supported targets for skill.json-less imports (claude, codex, claude,codex)")
	importCmd.Flags().BoolVar(&importCommit, "commit", false, "create a commit for the imported skill")
	rootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:     "import <path>",
	Short:   "Import a local skill directory into the repo",
	GroupID: "repo",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		targets, err := parseImportTargets(importTargets)
		if err != nil {
			return err
		}

		result, err := svc.ImportPath(args[0], targets, importCommit)
		if err != nil {
			return err
		}

		fmt.Printf("Imported %s to %s\n", result.SkillName, result.DestDir)
		if result.CommitCreated {
			fmt.Printf("Committed %s\n", result.SkillName)
		}
		return nil
	},
}

func parseImportTargets(raw string) ([]domain.Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	targets := make([]domain.Target, 0, len(parts))
	seen := make(map[domain.Target]bool)
	for _, part := range parts {
		target, err := domain.ParseTarget(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		if !seen[target] {
			targets = append(targets, target)
			seen[target] = true
		}
	}
	return targets, nil
}
