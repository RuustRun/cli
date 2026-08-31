// This file implements 'ruust login': it collects an email and password (from
// flags for non-interactive use, or by prompting), exchanges them for a session
// token via the API, saves the token and email to the config, and prints a
// styled welcome with the ember logo.
package commands

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/ui"
)

// loginEmailFlag and loginPasswordFlag back the --email and --password flags,
// which let scripts and CI sign in without an interactive prompt. loginNoBrowser
// keeps the browser flow but prints the URL instead of opening it automatically.
var (
	loginEmailFlag    string
	loginPasswordFlag string
	loginNoBrowser    bool
	loginHostFlag     string
)

// normaliseHost trims a supplied control-plane URL and defaults the scheme to https,
// so `--host ruust.run` and `--host https://ruust.run/` both work.
func normaliseHost(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return h
	}
	if !strings.HasPrefix(h, "http://") && !strings.HasPrefix(h, "https://") {
		h = "https://" + h
	}
	return strings.TrimRight(h, "/")
}

// loginCmd signs the user in and stores the returned session token.
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to Ruust",
	Long: "Sign in to Ruust. By default this opens your browser: you sign in on the\n" +
		"website and approve this computer, so no password is ever typed into the\n" +
		"terminal. The returned session token is saved to your config.\n\n" +
		"Pass --email and --password to sign in without a browser, handy for CI.\n" +
		"Pass --no-browser to print the sign-in link instead of opening it.\n" +
		"Pass --host to sign in to a specific control plane (default https://ruust.run);\n" +
		"the host is saved, so it also re-points a CLI stuck on an old (for example\n" +
		"localhost) host.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --host switches which control plane to sign in to and PERSISTS it, so a
		// stale stored host (for example a previous localhost dev login) cannot pin
		// later commands to the wrong place. Applied before the client is built and
		// saved with the session below.
		if loginHostFlag != "" {
			Config().Host = normaliseHost(loginHostFlag)
		}
		// A supplied --password means the non-interactive path (CI and scripts). The
		// default is the browser flow, so a password never travels through the terminal.
		if loginPasswordFlag != "" {
			return passwordLogin()
		}
		return browserLogin(loginNoBrowser)
	},
}

// passwordLogin signs in with an email and password (from flags, or prompted). It
// is the non-interactive path for CI and scripts; the default is the browser flow.
func passwordLogin() error {
	email := strings.TrimSpace(loginEmailFlag)
	password := loginPasswordFlag

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
		return errors.New("a password is required to sign in")
	}

	resp, err := Client().Login(email, password)
	if err != nil {
		return err
	}
	confirmed := resp.Email
	if confirmed == "" {
		confirmed = email
	}
	return saveSession(resp.Token, confirmed)
}

// browserLogin runs the loopback browser flow: it starts a local server on a
// random loopback port, opens the browser to the authorise page (carrying that
// port and a random state), waits for the control plane to redirect back with a
// single-use code, exchanges the code for a session token, and saves it. No
// password is entered in the terminal, and the token never travels through the URL.
func browserLogin(noBrowser bool) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting the local sign-in server: %w", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	state, err := randomState()
	if err != nil {
		return err
	}

	host := strings.TrimRight(Client().Host, "/")
	authURL := fmt.Sprintf("%s/cli/auth?port=%d&state=%s", host, port, url.QueryEscape(state))

	type result struct {
		code string
		err  error
	}
	ch := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// The state must match the one this process generated: it ties the callback
		// to our own request and blocks a stray or forged callback.
		if q.Get("state") != state {
			writeResultPage(w, "Sign-in could not be verified",
				"The sign-in did not match this session. Please run ruust login again.")
			ch <- result{err: errors.New("the sign-in state did not match; please try again")}
			return
		}
		if e := q.Get("error"); e != "" {
			writeResultPage(w, "Sign-in cancelled", "You can close this tab and return to your terminal.")
			ch <- result{err: errors.New("authorisation was declined in the browser")}
			return
		}
		code := q.Get("code")
		if code == "" {
			writeResultPage(w, "Sign-in failed", "No code was returned. Please run ruust login again.")
			ch <- result{err: errors.New("no authorisation code was returned")}
			return
		}
		writeResultPage(w, "You are signed in", "You can close this tab and return to your terminal.")
		ch <- result{code: code}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	fmt.Fprintln(os.Stderr, ui.Subtle.Render("Opening your browser to sign in..."))
	fmt.Fprintln(os.Stderr, ui.Key.Render(authURL))
	if noBrowser {
		fmt.Fprintln(os.Stderr, ui.Subtle.Render("Open the link above in your browser to continue."))
	} else if err := openInBrowser(authURL); err != nil {
		fmt.Fprintln(os.Stderr, ui.Subtle.Render("Could not open a browser automatically; open the link above."))
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return res.err
		}
		resp, err := Client().ExchangeCliCode(res.code)
		if err != nil {
			return err
		}
		return saveSession(resp.Token, resp.Email)
	case <-time.After(3 * time.Minute):
		return errors.New("timed out waiting for the browser sign-in")
	}
}

// randomState returns a high-entropy, URL-safe state string for the browser flow.
func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating a sign-in state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// writeResultPage renders a small, self-contained page shown in the browser after
// the loopback callback, telling the user to return to the terminal.
func writeResultPage(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="en-GB"><head><meta charset="utf-8">`+
		`<meta name="viewport" content="width=device-width, initial-scale=1"><title>Ruust CLI</title>`+
		`<style>body{margin:0;min-height:100vh;display:grid;place-items:center;`+
		`background:#0a0812;color:#efeafc;font-family:system-ui,sans-serif}`+
		`.card{max-width:26rem;padding:2rem;text-align:center}h1{font-size:1.3rem;margin:.4rem 0}`+
		`p{color:#b7add6;line-height:1.6}</style></head>`+
		`<body><div class="card"><div style="font-size:2rem">🥚</div><h1>%s</h1><p>%s</p></div></body></html>`,
		title, message)
}

// saveSession persists the session token and email, then prints the welcome.
func saveSession(token, email string) error {
	c := Config()
	c.Token = token
	if email != "" {
		c.Email = email
	}
	if err := c.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	printLoginSuccess(c.Email)
	return nil
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
		"Email to sign in with, for the non-interactive (CI) path")
	loginCmd.Flags().StringVar(&loginPasswordFlag, "password", "",
		"Password for the non-interactive (CI) path; the default is a browser sign-in")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false,
		"Print the sign-in link instead of opening a browser")
	loginCmd.Flags().StringVar(&loginHostFlag, "host", "",
		"Control plane to sign in to (default https://ruust.run); saved for later commands")
	AddCommand(loginCmd)
}
