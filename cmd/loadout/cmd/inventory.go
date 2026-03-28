package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/reconcile"
	"github.com/sethdeckard/loadout/internal/scope"
)

func init() {
	addProjectFlag(inventoryCmd)
	rootCmd.AddCommand(inventoryCmd)
}

var inventoryCmd = &cobra.Command{
	Use:     "inventory",
	Short:   "List skills and their status",
	GroupID: "skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		if projectFlag != "" {
			sc, err := scope.Resolve(projectFlag)
			if err != nil {
				return err
			}
			return projectInventory(svc, sc.Project)
		}

		views, err := svc.ListSkills()
		if err != nil {
			return err
		}

		if len(views) == 0 {
			fmt.Println("No skills found in registry.")
			return nil
		}

		for _, v := range views {
			claude := "[ ]"
			if v.InstalledClaude {
				claude = "[x]"
			}
			codex := "[ ]"
			if v.InstalledCodex {
				codex = "[x]"
			}

			var flags []string
			for _, f := range v.Flags {
				flags = append(flags, string(f))
			}
			flagStr := strings.Join(flags, ", ")
			if flagStr == string(reconcile.StatusInactive) {
				flagStr = ""
			}
			if flagStr != "" {
				flagStr = "  (" + flagStr + ")"
			}

			fmt.Printf("  %s C %s X  %-24s%s\n", claude, codex, v.Skill.Name, flagStr)
		}

		return nil
	},
}

func projectInventory(svc *app.Service, projectRoot string) error {
	views, err := svc.ProjectList(projectRoot)
	if err != nil {
		return err
	}

	if len(views) == 0 {
		fmt.Printf("No skills equipped in %s\n", projectRoot)
		return nil
	}

	fmt.Printf("Project: %s\n\n", projectRoot)
	for _, v := range views {
		claude := " "
		if v.ProjectClaude {
			claude = "C"
		}
		codex := " "
		if v.ProjectCodex {
			codex = "X"
		}
		fmt.Printf("  [%s%s]  %s\n", claude, codex, v.Skill.Name)
	}
	return nil
}
