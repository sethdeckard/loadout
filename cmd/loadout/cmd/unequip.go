package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/scope"
)

var unequipTarget string

func init() {
	unequipCmd.Flags().StringVar(&unequipTarget, "target", "", "target to unequip from (claude, codex)")
	addProjectFlag(unequipCmd)
	_ = unequipCmd.MarkFlagRequired("target")
	rootCmd.AddCommand(unequipCmd)
}

var unequipCmd = &cobra.Command{
	Use:     "unequip <skill-name>",
	Short:   "Unequip a skill from a target",
	GroupID: "skills",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := domain.ParseTarget(unequipTarget)
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
			if err := svc.ProjectRemove(id, target, sc.Project); err != nil {
				return err
			}
			fmt.Printf("Unequipped %s from %s in %s\n", id, target, sc.Project)
			return nil
		}

		if err := svc.DisableSkillTarget(id, target); err != nil {
			return err
		}

		fmt.Printf("Unequipped %s from %s\n", id, target)
		return nil
	},
}
