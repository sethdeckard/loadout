package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/scope"
)

var equipTarget string

func init() {
	equipCmd.Flags().StringVar(&equipTarget, "target", "", "target to equip for (claude, codex)")
	addProjectFlag(equipCmd)
	_ = equipCmd.MarkFlagRequired("target")
	rootCmd.AddCommand(equipCmd)
}

var equipCmd = &cobra.Command{
	Use:     "equip <skill-name>",
	Short:   "Equip a skill for a target",
	GroupID: "skills",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := domain.ParseTarget(equipTarget)
		if err != nil {
			return err
		}

		svc, err := newService()
		if err != nil {
			return err
		}

		id := domain.SkillName(args[0])

		if projectFlag != "" {
			sc, err := scope.Resolve(projectFlag)
			if err != nil {
				return err
			}
			if err := svc.ProjectInstall(id, target, sc.Project); err != nil {
				return err
			}
			fmt.Printf("Equipped %s for %s in %s\n", id, target, sc.Project)
			return nil
		}

		if err := svc.EnableSkillTarget(id, target); err != nil {
			return err
		}

		fmt.Printf("Equipped %s for %s\n", id, target)
		return nil
	},
}
