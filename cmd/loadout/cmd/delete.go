package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/scope"
)

var (
	deleteCommit bool
	deleteForce  bool
)

func init() {
	deleteCmd.Flags().BoolVar(&deleteCommit, "commit", false, "create a commit for the deletion")
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "skip interactive confirmation")
	addProjectFlag(deleteCmd)
	rootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:     "delete <skill-name>",
	Short:   "Delete a skill from the repo",
	GroupID: "repo",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		id := domain.SkillName(args[0])

		projectRoot := ""
		if projectFlag != "" {
			sc, err := scope.Resolve(projectFlag)
			if err != nil {
				return err
			}
			projectRoot = sc.Project
		}

		elig, err := svc.DeleteSkillEligibility(id, projectRoot)
		if err != nil {
			return err
		}
		if !elig.Deletable {
			for _, b := range elig.Blockers {
				fmt.Fprintf(os.Stderr, "  %s\n", b)
			}
			return fmt.Errorf("cannot delete %s", id)
		}

		if !deleteForce {
			fi, err := os.Stdin.Stat()
			if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
				return fmt.Errorf("confirmation cancelled: not an interactive terminal (use --force to skip)")
			}
			fmt.Printf("Type %q to confirm deletion: ", id)
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("confirmation cancelled")
			}
			if strings.TrimSpace(scanner.Text()) != string(id) {
				return fmt.Errorf("confirmation did not match, aborting")
			}
		}

		autoCommit := svc.Config.RepoActions.DeleteAutoCommit
		if cmd.Flags().Changed("commit") {
			autoCommit = deleteCommit
		}

		result, err := svc.DeleteSkill(id, projectRoot, autoCommit)
		if err != nil {
			return err
		}

		fmt.Printf("Deleted %s from repo\n", result.SkillName)
		if result.CommitCreated {
			fmt.Printf("Committed %s\n", result.SkillName)
		}
		return nil
	},
}
