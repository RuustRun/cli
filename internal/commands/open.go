// This file implements the 'ruust open <name>' command: it resolves an Egg by
// name and opens its url in the default browser, using the OS opener ('open' on
// darwin, 'xdg-open' on linux).
package commands

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

// openCmd opens an Egg's url in the browser.
var openCmd = &cobra.Command{
	Use:   "open <name>",
	Short: "Open an Egg's url in your browser",
	Long: "Resolve an Egg by name and open https://<url> in your default browser.\n" +
		"Handy for jumping straight to a hatched Egg.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return errNotSignedIn
		}

		name := args[0]
		egg, err := resolveEggByName(Client(), name)
		if err != nil {
			return err
		}

		if strings.TrimSpace(egg.URL) == "" {
			return fmt.Errorf("Egg %q has no url yet (it may still be %s)",
				egg.Name, strings.ToLower(egg.State))
		}

		target := "https://" + strings.TrimSpace(egg.URL)
		fmt.Fprintln(cmd.OutOrStdout(), ui.Subtle.Render("Opening ")+ui.Ember.Render(target)+
			ui.Subtle.Render(" for Egg ")+ui.Key.Render(egg.Name))

		if err := openInBrowser(target); err != nil {
			return fmt.Errorf("could not open your browser: %w", err)
		}
		return nil
	},
}

// openInBrowser launches the OS default browser at url. It supports darwin and
// linux (and falls back to a helpful message elsewhere).
func openInBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{url}
	case "linux":
		name = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("opening a browser is not supported on %s; visit %s yourself",
			runtime.GOOS, url)
	}

	return exec.Command(name, args...).Start()
}

func init() {
	AddCommand(openCmd)
}
