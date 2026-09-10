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

// logsBuildFlag and logsReleaseFlag switch from runtime logs to the latest deploy's
// build output, or its release-command (e.g. migrations) output.
var (
	logsBuildFlag   bool
	logsReleaseFlag bool
)

// logsCmd tails the logs for an Egg by name.
var logsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Show the logs for an Egg",
	Long: "logs resolves an Egg by name and prints its most recent runtime log lines,\n" +
		"timestamped and level-coloured like a terminal log tail.\n\n" +
		"Use --build for the latest deploy's build output, or --release for its\n" +
		"release-command output (e.g. database migrations run before the roll).",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return fmt.Errorf("not signed in (run 'ruust login')")
		}
		if logsBuildFlag && logsReleaseFlag {
			return fmt.Errorf("use only one of --build or --release")
		}

		name := strings.TrimSpace(args[0])
		client := Client()

		egg, err := resolveEggByName(client, name)
		if err != nil {
			return err
		}

		if logsBuildFlag || logsReleaseFlag {
			bl, err := client.BuildLog(egg.ID)
			if err != nil {
				return err
			}
			if logsReleaseFlag {
				printReleaseLog(egg, bl)
			} else {
				printBuildLog(egg, bl)
			}
			return nil
		}

		res, err := client.Logs(egg.ID)
		if err != nil {
			return err
		}

		printLogs(egg, res.Lines, logsLinesFlag)
		return nil
	},
}

// deref returns the pointed-to string, or "" for nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// printBuildLog renders the latest deploy's build output as a plain block.
func printBuildLog(egg api.Egg, bl api.BuildLogResponse) {
	head := ui.Title.Render("build") + " " + ui.Ember.Render(egg.Name)
	if sha := deref(bl.GitSha); sha != "" {
		head += " " + ui.Subtle.Render(shortSha(sha))
	}
	if st := deref(bl.Status); st != "" {
		head += " " + ui.Subtle.Render("("+st+")")
	}
	fmt.Println(head)

	if body := deref(bl.Log); body != "" {
		fmt.Println(printableLog(body))
		return
	}
	fmt.Println(ui.Subtle.Render("No build output (a database or demo Egg runs a prebuilt image)."))
}

// printReleaseLog renders the release-command outcome and captured output for the
// latest deploy (e.g. the migrations run before the roll).
func printReleaseLog(egg api.Egg, bl api.BuildLogResponse) {
	head := ui.Title.Render("release") + " " + ui.Ember.Render(egg.Name)
	if st := deref(bl.ReleaseStatus); st != "" {
		head += " " + ui.Subtle.Render("· "+st)
	}
	if v := deref(bl.ReleaseAgentVersion); v != "" {
		head += " " + ui.Subtle.Render("· agent "+v)
	}
	fmt.Println(head)

	switch {
	case deref(bl.ReleaseStatus) == "" && deref(bl.ReleaseLog) == "":
		fmt.Println(ui.Subtle.Render("No release step for this deploy."))
	case deref(bl.ReleaseLog) != "":
		fmt.Println(printableLog(deref(bl.ReleaseLog)))
	default:
		fmt.Println(ui.Subtle.Render("Release " + deref(bl.ReleaseStatus) + " with no output."))
	}
}

// printableLog trims trailing whitespace and renders a raw multi-line log body in
// the mono log styling, dropping empty lines so the output reads cleanly.
func printableLog(body string) string {
	var b strings.Builder
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if ln == "" {
			continue
		}
		b.WriteString(mono.Render(ln))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// shortSha trims a git sha to its first seven characters for display.
func shortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
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
	logsCmd.Flags().BoolVar(&logsBuildFlag, "build", false,
		"show the latest deploy's build output instead of runtime logs")
	logsCmd.Flags().BoolVar(&logsReleaseFlag, "release", false,
		"show the latest deploy's release-command output (e.g. migrations)")
	AddCommand(logsCmd)
}
