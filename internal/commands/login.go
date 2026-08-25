// This file implements 'ruust login': it collects an email and password (from
// flags for non-interactive use, or by prompting), exchanges them for a session
// token via the API, saves the token and email to the config, and prints a
// styled welcome with the ember logo.
package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/ui"
)

// loginEmailFlag and loginPasswordFlag back the --email and --password flags,
// which let scripts and CI sign in without an interactive prompt.
var (
	loginEmailFlag    string
	loginPasswordFlag string
)

// loginCmd signs the user in and stores the returned session token.
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to Ruust",
	Long: "Sign in to Ruust with your email and password. The returned session\n" +
		"token is saved to your config so the other commands can use it.\n\n" +
		"Pass --email and --password to sign in without a prompt, handy for CI.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		email := strings.TrimSpace(loginEmailFlag)
		password := loginPasswordFlag

		// Prompt for anything the flags did not supply.
		if email == "" {
			e, err := promptLine("Email: ")
			if err != nil {
				return err
			}
			email = strings.TrimSpace(e)
		}
		if email == "" {
			return errors.New("an email is required to sign in")
		}

		if password == "" {
			p, err := promptPassword("Password: ")
			if err != nil {
				return err
			}
			password = p
		}
		if password == "" {
			return errors.New("a password is required to sign in")
		}

		resp, err := Client().Login(email, password)
		if err != nil {
			return err
		}

		// Persist the token and the email the server confirmed. Fall back to the
		// email we sent if the server did not echo one back.
		c := Config()
		c.Token = resp.Token
		if resp.Email != "" {
			c.Email = resp.Email
		} else {
			c.Email = email
		}
		if err := c.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		printLoginSuccess(c.Email)
		return nil
	},
}

// promptLine writes a prompt to stderr and reads a single line from stdin. The
// prompt goes to stderr so piped stdout stays clean for scripting.
func promptLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, ui.Key.Render(prompt))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// promptPassword reads a password from stdin without echoing it to the terminal.
//
// It disables terminal echo with stty for the duration of the read, then
// restores it, so the typed password never appears on screen. When stdin is not
// a terminal (a pipe or here-doc, as in CI) it reads the line plainly, since
// there is no echo to hide.
func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, ui.Key.Render(prompt))

	restore, hidden := hideInput()
	if hidden {
		defer restore()
	}

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')

	// The suppressed newline the user pressed is not echoed, so add one so the
	// next line of output starts cleanly.
	if hidden {
		fmt.Fprintln(os.Stderr)
	}

	if err != nil && line == "" {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// hideInput turns off terminal echo when stdin is an interactive terminal. It
// returns a restore function and whether echo was actually disabled. When stdin
// is not a terminal it is a no-op, so piped input still works.
func hideInput() (restore func(), hidden bool) {
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return func() {}, false
	}

	saved, err := sttyState()
	if err != nil {
		return func() {}, false
	}
	if err := runStty("-echo"); err != nil {
		return func() {}, false
	}
	return func() { _ = runStty(saved) }, true
}

// sttyState captures the current terminal settings so they can be restored after
// echo is toggled off.
func sttyState() (string, error) {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// runStty applies one or more stty arguments to the controlling terminal.
func runStty(arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// printLoginSuccess prints the ember logo and a warm welcome confirming who is
// now signed in.
func printLoginSuccess(email string) {
	fmt.Println(ui.Logo())
	fmt.Println()
	fmt.Println(ui.Title.Render("Welcome back.") + " " +
		ui.Bone.Render("You are signed in as ") + ui.Ember.Render(email) + ui.Bone.Render("."))
	fmt.Println(ui.Subtle.Render("Run ") + ui.Key.Render("ruust") +
		ui.Subtle.Render(" to open your Eggs."))
}

func init() {
	loginCmd.Flags().StringVar(&loginEmailFlag, "email", "",
		"Email to sign in with (prompts when omitted)")
	loginCmd.Flags().StringVar(&loginPasswordFlag, "password", "",
		"Password to sign in with (prompts without echo when omitted)")
	AddCommand(loginCmd)
}
