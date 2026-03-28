package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Health check for loadout configuration",
	GroupID: "setup",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		report, err := svc.Doctor()
		if err != nil {
			return err
		}

		for _, c := range report.Checks {
			status := "OK"
			if !c.OK {
				status = "!!"
			}
			fmt.Printf("  [%s] %-16s %s\n", status, c.Name, c.Detail)
		}

		if report.AllOK {
			fmt.Println("\nAll checks passed.")
		} else {
			fmt.Println("\nSome checks failed.")
		}

		return nil
	},
}
