package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/ui"
)

// eggMark is the little ember egg glyph that leads each row.
const eggMark = "\U0001F95A" // egg

// listView renders the styled list of Eggs in the ember palette.
func (m model) listView() string {
	header := m.listHeader()
	body := m.listBody()
	footer := m.footer([]key.Binding{
		m.keys.Up,
		m.keys.Down,
		m.keys.Open,
		m.keys.Refresh,
		m.keys.Quit,
	})

	// Fill the middle so the footer sits at the bottom.
	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}
	middle := lipgloss.NewStyle().
		Width(m.width).
		Height(contentHeight).
		Render(body)

	return lipgloss.JoinVertical(lipgloss.Left, header, middle, footer)
}

// listHeader renders the ruust logo, a subtitle, and an Egg count.
func (m model) listHeader() string {
	logo := ui.Logo() + "  " + ui.Subtle.Render("the nest")

	count := len(m.eggs)
	noun := "Eggs"
	if count == 1 {
		noun = "Egg"
	}
	total := m.monthlyTotal()
	countLabel := ui.Ember.Render(fmt.Sprintf("%d", count)) + " " + ui.Subtle.Render(noun)
	if total > 0 {
		countLabel += ui.Subtle.Render("  •  ") +
			ui.Key.Render(fmt.Sprintf("£%g", total)) + ui.Subtle.Render("/mo")
	}

	gap := m.width - lipgloss.Width(logo) - lipgloss.Width(countLabel) - 4
	if gap < 1 {
		gap = 1
	}
	row := logo + strings.Repeat(" ", gap) + countLabel

	rule := lipgloss.NewStyle().
		Foreground(ui.ColourEmber).
		Render(strings.Repeat("─", max(m.width-4, 1)))

	return lipgloss.NewStyle().Padding(1, 2, 0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, row, rule),
	)
}

// monthlyTotal sums the flat GBP price across all Eggs.
func (m model) monthlyTotal() float64 {
	var t float64
	for _, e := range m.eggs {
		t += e.PriceGbp
	}
	return t
}

// listBody renders the rows, or an empty-nest note when there are no Eggs.
func (m model) listBody() string {
	if len(m.eggs) == 0 {
		empty := lipgloss.JoinVertical(lipgloss.Center,
			ui.Subtle.Render("your nest is empty"),
			"",
			ui.Subtle.Render("deploy your first Egg with ")+ui.Key.Render("ruust deploy"),
		)
		return lipgloss.NewStyle().
			Width(m.width).
			Padding(2, 2).
			Render(empty)
	}

	rows := make([]string, 0, len(m.eggs))
	for i, e := range m.eggs {
		rows = append(rows, m.eggRow(e, i == m.cursor))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)
}

// eggRow renders a single Egg as a row of left-aligned, fixed-width columns:
// mark, name, region, tier and a coloured lifecycle state. Columns are sized to
// the widest value present so the rows line up tidily. The selected row is
// highlighted with an ember bar and a dim background.
func (m model) eggRow(e api.Egg, selected bool) string {
	nameW, regionW, tierW := m.colWidths()

	pad := func(s string, w int) string {
		return lipgloss.NewStyle().Width(w).Render(s)
	}
	dot := ui.Subtle.Render(" · ")

	name := ui.Bone.Bold(true).Render(truncate(e.Name, nameW))
	region := ui.Subtle.Render(regionText(e))
	tier := ui.Key.Render(tierText(e))
	state := ui.StateLabel(e.State)

	line := eggMark + "  " + pad(name, nameW) + "    " +
		pad(region, regionW) + dot + pad(tier, tierW) + dot + state

	if selected {
		bar := lipgloss.NewStyle().Foreground(ui.ColourEmber).Render("▎")
		return lipgloss.NewStyle().
			Width(m.width-4).
			Background(ui.ColourDimBg).
			Padding(0, 1).
			Render(bar + " " + line)
	}
	return lipgloss.NewStyle().
		Width(m.width-4).
		Padding(0, 1).
		Render("  " + line)
}

// colWidths sizes the name, region and tier columns to the widest value present,
// so the rows line up. Names are capped so one long name cannot blow out the row.
func (m model) colWidths() (name, region, tier int) {
	name, region, tier = 4, 6, 4
	for _, e := range m.eggs {
		if l := lipgloss.Width(truncate(e.Name, 24)); l > name {
			name = l
		}
		if l := lipgloss.Width(regionText(e)); l > region {
			region = l
		}
		if l := lipgloss.Width(tierText(e)); l > tier {
			tier = l
		}
	}
	return name, region, tier
}

// regionText renders the region label with its slug in parentheses.
func regionText(e api.Egg) string {
	label := e.RegionLabel
	if label == "" {
		label = e.Region
	}
	if e.Region != "" && label != e.Region {
		return fmt.Sprintf("%s (%s)", label, e.Region)
	}
	return label
}

// tierText renders the tier label, falling back to the slug.
func tierText(e api.Egg) string {
	if e.TierLabel != "" {
		return e.TierLabel
	}
	return e.Tier
}

// truncate shortens s to n runes, appending an ellipsis when it overflows.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
