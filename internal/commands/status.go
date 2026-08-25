// This file owns the 'ruust status' command. It fetches the deployment regions
// and prints their availability as a styled list with live and soon dots, under
// an "All systems operational" header. Regions are independent cells, so a
// wobble in one never touches another, and the output makes that plain.
package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

// statusCmd prints region availability across Ruust.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Ruust region availability",
	Long: "status lists the Ruust regions and whether each is live or coming soon.\n" +
		"Every region is an independent cell, so trouble in one never spreads to another.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return fmt.Errorf("not signed in (run 'ruust login')")
		}

		regions, err := Client().Regions()
		if err != nil {
			return err
		}

		printStatus(regions)
		return nil
	},
}

// liveDot is the green dot for a live region.
var liveDot = lipgloss.NewStyle().Foreground(ui.ColourLive).Render("●")

// soonDot is the amber dot for a region coming soon.
var soonDot = lipgloss.NewStyle().Foreground(ui.ColourWarn).Render("●")

// printStatus renders the operational header, the region list, and a closing
// note that regions are independent cells.
func printStatus(regions []api.Region) {
	fmt.Println(ui.Logo())
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(ui.ColourLive).Bold(true).
		Render("All systems operational"))
	fmt.Println()

	if len(regions) == 0 {
		fmt.Println(ui.Subtle.Render("No regions reported."))
		return
	}

	for _, r := range regions {
		fmt.Println(renderRegion(r))
	}

	fmt.Println()
	fmt.Println(ui.Subtle.Render("Every region is an independent cell, isolated from the others."))
}

// renderRegion renders one region row: an availability dot, the display name,
// the region slug in muted text, and a live or soon tag.
func renderRegion(r api.Region) string {
	live := strings.EqualFold(strings.TrimSpace(r.Availability), "live")

	dot := soonDot
	tag := lipgloss.NewStyle().Foreground(ui.ColourWarn).Bold(true).Render("soon")
	if live {
		dot = liveDot
		tag = lipgloss.NewStyle().Foreground(ui.ColourLive).Bold(true).Render("live")
	}

	name := ui.Bone.Render(r.DisplayName)
	slug := ui.Subtle.Render("(" + r.Slug + ")")

	return fmt.Sprintf("  %s  %s %s  %s", dot, name, slug, tag)
}

func init() {
	AddCommand(statusCmd)
}
