// This file implements 'ruust logout': it clears the stored session token (and
// the remembered email) from the config so the CLI is signed out.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/ui"
)

// logoutCmd clears the saved session token from the config.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of Ruust",
	Long: "Sign out by clearing the saved session token from your config. Your Eggs\n" +
		"keep running; this only forgets the token on this machine.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := Config()

		// Nothing stored means we are already signed out.
		if c.Token == "" && c.Email == "" {
			fmt.Println(ui.Subtle.Render("You are already signed out."))
			return nil
		}

		c.Token = ""
		c.Email = ""
		if err := c.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Println(ui.Bone.Render("Signed out. ") +
			ui.Subtle.Render("Run ") + ui.Key.Render("ruust login") +
			ui.Subtle.Render(" to sign back in."))
		return nil
	},
}

func init() {
	AddCommand(logoutCmd)
}
