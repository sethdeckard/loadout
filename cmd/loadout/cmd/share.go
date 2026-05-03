package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/domain"
)

var shareOut string

func init() {
	shareCmd.Flags().StringVar(&shareOut, "out", "", "output path: directory (writes <name>.tar.gz inside) or full archive path")
	rootCmd.AddCommand(shareCmd)
}

var shareCmd = &cobra.Command{
	Use:     "share <skill-name>",
	Short:   "Package a skill into a portable .tar.gz archive",
	GroupID: "skills",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		archivePath, err := svc.Share(domain.SkillName(args[0]), shareOut)
		if err != nil {
			return err
		}

		fmt.Printf("Wrote %s\n", archivePath)
		return nil
	},
}
