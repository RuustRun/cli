// This file owns the 'ruust shell' command: an interactive shell inside an Egg's
// container. It mints a session from the control plane, opens the relay WebSocket, puts
// the local terminal in raw mode, and bridges keystrokes and terminal output, exactly
// like `docker exec -it` but to a container running on a Ruust host.
package commands

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

var shellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open an interactive shell in an Egg's container",
	Long: "shell resolves an Egg by name and opens an interactive shell in its running\n" +
		"container, like 'docker exec -it', over a secure relay. Requires a terminal.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return fmt.Errorf("not signed in (run 'ruust login')")
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("shell needs an interactive terminal")
		}

		name := strings.TrimSpace(args[0])
		client := Client()
		egg, err := resolveEggByName(client, name)
		if err != nil {
			return err
		}
		sess, err := client.Shell(egg.ID)
		if err != nil {
			return err
		}

		fmt.Println(ui.Subtle.Render("connecting to " + egg.Name + "..."))
		return runShell(sess)
	},
}

// runShell opens the relay WebSocket and bridges the local terminal to the remote PTY.
func runShell(sess api.ShellSession) error {
	conn, _, err := websocket.DefaultDialer.Dial(sess.WsURL, nil)
	if err != nil {
		return fmt.Errorf("connect to shell relay: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// gorilla connections are not safe for concurrent writers; serialise all writes.
	var writeMu sync.Mutex
	writeBinary := func(p []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.BinaryMessage, p)
	}
	sendResize := func() {
		w, h, e := term.GetSize(int(os.Stdin.Fd()))
		if e != nil {
			return
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteJSON(map[string]any{"type": "resize", "cols": w, "rows": h})
	}

	// Raw mode so keystrokes go straight through; restored on exit.
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("set raw terminal: %w", err)
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	// Initial window size, then track resizes.
	sendResize()
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			sendResize()
		}
	}()

	// stdin -> WS (binary). A read error or write error ends the session.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := os.Stdin.Read(buf)
			if n > 0 {
				if werr := writeBinary(buf[:n]); werr != nil {
					_ = conn.Close()
					return
				}
			}
			if rerr != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	// WS -> stdout; the agent sends {"type":"exit"} (text) when the shell exits.
	for {
		mt, data, rerr := conn.ReadMessage()
		if rerr != nil {
			return nil
		}
		switch mt {
		case websocket.BinaryMessage:
			_, _ = os.Stdout.Write(data)
		case websocket.TextMessage:
			if strings.Contains(string(data), `"exit"`) {
				return nil
			}
		}
	}
}

func init() {
	AddCommand(shellCmd)
}
