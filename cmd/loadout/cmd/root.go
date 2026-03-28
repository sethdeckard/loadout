package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/scope"
	"github.com/sethdeckard/loadout/internal/tui"
)

var (
	projectFlag string
	userFlag    bool
)

func init() {
	rootCmd.Flags().BoolVar(&userFlag, "user", false, "start the TUI in user scope even inside a project")
	rootCmd.AddGroup(
		&cobra.Group{ID: "skills", Title: "Skills:"},
		&cobra.Group{ID: "repo", Title: "Repo:"},
		&cobra.Group{ID: "setup", Title: "Setup:"},
	)
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("loadout {{.Version}}\n")
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SetHelpCommandGroupID("setup")
	rootCmd.SetUsageTemplate(strings.Replace(rootCmd.UsageTemplate(),
		`{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}`,
		`
  {{if .HasParent}}{{.UseLine}}{{else}}{{.CommandPath}}             Launch the TUI
  {{.CommandPath}} [command]   Run a CLI command{{end}}`,
		1))
}

var rootCmd = &cobra.Command{
	Use:   "loadout",
	Short: "Skill manager for Claude and Codex",
	Long:  "\n" + tui.Logo + "\n\n  Manages machine-local skill loadouts for Claude and Codex\n  from a git repo you own.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.DefaultPath()
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			fmt.Println("\n" + tui.Logo)
			if err := runInitWith(os.Stdout, os.Stdin); err != nil {
				return err
			}
			fmt.Println()
		}
		svc, err := newService()
		if err != nil {
			return err
		}
		return runTUI(svc, detectProject(), userFlag)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newService() (*app.Service, error) {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config from %s: %w\nRun 'loadout init' to set up", cfgPath, err)
	}
	return app.New(cfg), nil
}

var runTUI = func(svc *app.Service, projectRoot string, userScope bool) error {
	p := tea.NewProgram(tui.NewModel(svc, projectRoot, userScope), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func detectProject() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	root, err := scope.DetectProjectRoot(wd)
	if err != nil {
		return ""
	}
	return root
}

func addProjectFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&projectFlag, "project", "", "project path (uses CWD git root if \".\")")
}
