package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/domain"
)

func init() {
	rootCmd.AddCommand(inspectCmd)
}

var inspectCmd = &cobra.Command{
	Use:     "inspect <skill-name>",
	Short:   "Preview skill details and metadata",
	GroupID: "skills",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		preview, err := svc.PreviewSkill(domain.SkillName(args[0]))
		if err != nil {
			return err
		}

		s := preview.Skill
		fmt.Printf("Name:        %s\n", s.Name)
		fmt.Printf("Description: %s\n", s.Description)
		fmt.Printf("Tags:        %v\n", s.Tags)
		fmt.Printf("Targets:     %v\n", s.Targets)
		fmt.Printf("Path:        %s\n", s.Path)
		fmt.Println()
		fmt.Println(preview.Markdown)

		return nil
	},
}
