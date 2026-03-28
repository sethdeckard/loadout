package cmd

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("%s (%s, %s)", version, commit, date)
}
