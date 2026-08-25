// This file implements 'ruust whoami': it asks the control plane who the current
// session belongs to and prints the signed-in email, nudging the user to sign in
// when there is no valid session.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

// whoamiCmd prints the currently signed-in account.
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show who you are signed in as",
	Long: "Show the email of the account tied to your current session. If you are\n" +
		"not signed in, it points you at 'ruust login'.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Without a token there is nothing to ask the server about, so hint early.
		if config.Token(Config()) == "" {
			printLoginHint()
			return nil
		}

		me, err := Client().Me()
		if err != nil {
			return err
		}

		fmt.Println(ui.Key.Render("Signed in as ") + ui.Ember.Render(me.Email))
		return nil
	},
}

func init() {
	AddCommand(whoamiCmd)
}
