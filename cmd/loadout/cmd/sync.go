package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/scope"
)

func init() {
	addProjectFlag(syncCmd)
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:     "sync",
	Short:   "Reconcile repo with remote and refresh outdated managed installs",
	GroupID: "repo",
	Long: "Sync the shared skill repo with its remote and refresh outdated managed installs.\n\n" +
		"Loadout pushes local repo commits when ahead, pulls remote commits when behind, " +
		"and then refreshes managed installs from the resolved repo HEAD.\n\n" +
		"Without --project, Loadout refreshes managed user installs. With --project, " +
		"Loadout also refreshes managed installs in that project's .claude/skills and .codex/skills roots.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		projectRoot := ""
		if projectFlag != "" {
			sc, err := scope.Resolve(projectFlag)
			if err != nil {
				return err
			}
			projectRoot = sc.Project
		}

		fmt.Println("Reconciling repository with remote and refreshing managed installs...")
		result, err := svc.SyncRepoWithResult(projectRoot)
		if err != nil {
			return err
		}

		fmt.Print("Sync complete.")
		if result.Bootstrapped {
			fmt.Print(" Published first local commit.")
		} else {
			if result.Pushed {
				fmt.Print(" Pushed repo changes.")
			}
			if result.Pulled {
				fmt.Print(" Pulled repo changes.")
			}
		}
		if result.RefreshedUser > 0 {
			fmt.Printf(" Refreshed %d user install(s).", result.RefreshedUser)
		}
		if result.RefreshedProject > 0 {
			fmt.Printf(" Refreshed %d project install(s).", result.RefreshedProject)
		}
		if !result.Bootstrapped && !result.Pushed && !result.Pulled && result.RefreshedTotal() == 0 {
			fmt.Print(" No repo or managed-install changes.")
		} else if result.RefreshedTotal() == 0 {
			fmt.Print(" Managed installs already current.")
		}
		fmt.Println()
		return nil
	},
}
