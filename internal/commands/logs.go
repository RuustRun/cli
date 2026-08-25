// This file owns the 'ruust logs' command. It resolves an Egg by name, fetches
// its log lines, and prints them like a terminal log tail: each line is
// timestamped, level-coloured, and rendered in a mono styling so the output
// reads as a live log stream.
package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

// logsLinesFlag holds the value of the --lines flag: the maximum number of most
// recent log lines to show.
var logsLinesFlag int

// logsCmd tails the logs for an Egg by name.
var logsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Show the logs for an Egg",
	Long: "logs resolves an Egg by name and prints its most recent log lines,\n" +
		"timestamped and level-coloured like a terminal log tail.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return fmt.Errorf("not signed in (run 'ruust login')")
		}

		name := strings.TrimSpace(args[0])
		client := Client()

		egg, err := resolveEggByName(client, name)
		if err != nil {
			return err
		}

		res, err := client.Logs(egg.ID)
		if err != nil {
			return err
		}

		printLogs(egg, res.Lines, logsLinesFlag)
		return nil
	},
}

// resolveEggByName finds the single Egg whose name matches, case-insensitively.
// It returns a friendly error when nothing matches or the name is ambiguous.
func resolveEggByName(client *api.Client, name string) (api.Egg, error) {
	eggs, err := client.ListEggs()
	if err != nil {
		return api.Egg{}, err
	}

	var matches []api.Egg
	for _, e := range eggs {
		if strings.EqualFold(strings.TrimSpace(e.Name), name) {
			matches = append(matches, e)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return api.Egg{}, fmt.Errorf("no Egg named %q (run 'ruust eggs' to list yours)", name)
	default:
		return api.Egg{}, fmt.Errorf("more than one Egg is named %q, please use a unique name", name)
	}
}

// printLogs renders the log lines as a terminal log tail. When limit is greater
// than zero only the last limit lines are shown.
func printLogs(egg api.Egg, lines []api.LogLine, limit int) {
	header := ui.Title.Render("logs") + " " +
		ui.Ember.Render(egg.Name) + " " +
		ui.Subtle.Render("("+egg.RegionLabel+")")
	fmt.Println(header)

	if len(lines) == 0 {
		fmt.Println(ui.Subtle.Render("No log lines yet."))
		return
	}

	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	for _, ln := range lines {
		fmt.Println(renderLogLine(ln))
	}
}

// mono is a terminal-style monospaced-feel log style for log body text.
var mono = lipgloss.NewStyle().Foreground(ui.ColourBone)

// logTimeStyle renders the timestamp column in muted tones.
var logTimeStyle = lipgloss.NewStyle().Foreground(ui.ColourMuted)

// renderLogLine renders a single log line: a muted timestamp, a fixed-width
// level tag coloured by severity, then the mono log text.
func renderLogLine(ln api.LogLine) string {
	ts := logTimeStyle.Render(formatLogTS(ln.TS))
	level := renderLogLevel(ln.Level)
	text := mono.Render(ln.Text)
	return ts + " " + level + " " + text
}

// formatLogTS renders an ISO timestamp as a compact local time of day. It falls
// back to the raw string when the timestamp cannot be parsed.
func formatLogTS(ts string) string {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Local().Format("15:04:05")
	}
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Local().Format("15:04:05")
	}
	return ts
}

// renderLogLevel returns a padded, colour-coded tag for a log level: info and
// debug are muted, warn is amber, and error and fatal are red.
func renderLogLevel(level string) string {
	norm := strings.ToUpper(strings.TrimSpace(level))
	tag := norm
	if tag == "" {
		tag = "LOG"
	}
	// Pad to a fixed width so the log body lines up in a neat column.
	tag = fmt.Sprintf("%-5s", tag)

	var colour lipgloss.Color
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error", "err", "fatal", "panic":
		colour = ui.ColourBad
	case "warn", "warning":
		colour = ui.ColourWarn
	case "debug", "trace":
		colour = ui.ColourMuted
	default:
		colour = ui.ColourEmberSoft
	}
	return lipgloss.NewStyle().Foreground(colour).Bold(true).Render(tag)
}

func init() {
	logsCmd.Flags().IntVar(&logsLinesFlag, "lines", 0,
		"limit output to the last N log lines (0 shows all returned)")
	AddCommand(logsCmd)
}
