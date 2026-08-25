// This file is a placeholder subcommand that also demonstrates the registration
// pattern the next-phase command files follow: define a *cobra.Command and call
// AddCommand from init(). It additionally references the bubbles widgets library
// so that dependency stays wired into the module graph for the next phase, where
// the real commands (login, eggs, deploy, logs, regions) will live.
package commands

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/ui"
)

// Version is the ruust CLI version. It is overridden at build time via ldflags.
var Version = "dev"

// versionCmd prints the ruust version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ruust version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ui.Logo() + " " + ui.Subtle.Render(Version))
		return nil
	},
}

// newSpinner returns a bubbles spinner styled in the ember colour. It is a small
// helper the next-phase commands and TUI can reuse for progress feedback while
// an Egg is incubating or hatching.
func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ui.Ember
	return s
}

func init() {
	AddCommand(versionCmd)
	// Keep the helper referenced so the bubbles dependency is retained until the
	// next-phase command files use it directly.
	_ = newSpinner
}
