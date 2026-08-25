// This file implements the 'ruust ls' command (and its 'eggs' alias, wired in
// eggs.go): it lists the Eggs the signed-in account owns as a lipgloss table
// with a coloured state label per Egg, then prints a summary line with counts
// and the flat monthly total in GBP.
package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/ui"
)

// lsCmd lists the Eggs the signed-in account owns.
var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"eggs", "list"},
	Short:   "List your Eggs",
	Long: "List every Egg you own as a table showing its name, region, tier,\n" +
		"lifecycle state, and url, with a summary of the flat monthly total.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			return errNotSignedIn
		}

		eggs, err := Client().ListEggs()
		if err != nil {
			return err
		}

		if len(eggs) == 0 {
			fmt.Println(ui.Logo())
			fmt.Println()
			fmt.Println(ui.Bone.Render("You have no Eggs yet."))
			fmt.Println(ui.Subtle.Render("Run ") + ui.Key.Render("ruust create --repo <git url>") +
				ui.Subtle.Render(" to hatch your first one."))
			return nil
		}

		printEggsTable(cmd.OutOrStdout(), eggs)
		return nil
	},
}

// errNotSignedIn is the shared error shown when no session token is present.
var errNotSignedIn = fmt.Errorf("not signed in (run 'ruust login')")

// printEggsTable renders the Eggs as a lipgloss table followed by a summary
// line with per-state counts and the flat monthly total.
func printEggsTable(w interface{ Write([]byte) (int, error) }, eggs []api.Egg) {
	// Keep output stable and readable: newest names last, sorted by name.
	sorted := make([]api.Egg, len(eggs))
	copy(sorted, eggs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})

	headerStyle := ui.Key.Copy().Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColourMuted)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers("NAME", "REGION", "TIER", "STATE", "URL")

	var total float64
	counts := map[string]int{}

	for _, e := range sorted {
		total += e.PriceGbp
		counts[strings.ToLower(strings.TrimSpace(e.State))]++

		t.Row(
			ui.Ember.Render(e.Name),
			regionCell(e),
			ui.Bone.Render(tierCell(e)),
			ui.StateLabel(e.State),
			ui.Subtle.Render(e.URL),
		)
	}

	fmt.Fprintln(w, t.Render())
	fmt.Fprintln(w)
	fmt.Fprintln(w, eggsSummary(len(sorted), counts, total))
}

// regionCell prefers the human region label, falling back to the slug.
func regionCell(e api.Egg) string {
	if strings.TrimSpace(e.RegionLabel) != "" {
		return ui.Bone.Render(e.RegionLabel)
	}
	return ui.Bone.Render(e.Region)
}

// tierCell prefers the human tier label, falling back to the tier slug.
func tierCell(e api.Egg) string {
	if strings.TrimSpace(e.TierLabel) != "" {
		return e.TierLabel
	}
	return e.Tier
}

// eggsSummary builds the summary line: total Egg count, a coloured breakdown by
// lifecycle state, and the flat monthly total in GBP.
func eggsSummary(n int, counts map[string]int, total float64) string {
	noun := "Eggs"
	if n == 1 {
		noun = "Egg"
	}

	var parts []string
	// Order the breakdown by the lifecycle, most lively first.
	for _, state := range []string{"hatched", "hatching", "incubating", "cold", "cracked"} {
		if c := counts[state]; c > 0 {
			parts = append(parts, ui.StateStyle(state).Render(fmt.Sprintf("%d %s", c, state)))
		}
	}

	head := ui.Key.Render(fmt.Sprintf("%d %s", n, noun))
	breakdown := ""
	if len(parts) > 0 {
		breakdown = ui.Subtle.Render(" (") + strings.Join(parts, ui.Subtle.Render(", ")) + ui.Subtle.Render(")")
	}
	money := ui.Ember.Render(formatGBP(total)) + ui.Subtle.Render(" per month")

	return head + breakdown + ui.Subtle.Render("  ·  ") + money
}

// formatGBP formats a GBP amount, dropping the decimals for whole pounds.
func formatGBP(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("£%d", int64(amount))
	}
	return fmt.Sprintf("£%.2f", amount)
}

func init() {
	AddCommand(lsCmd)
}
