// Package ui holds the shared lipgloss styles for the ruust TUI and command
// output: the nocturnal ember palette, egg lifecycle colours and labels, the
// ruust logo, and a small resource bar renderer.
//
// Customer-facing vocabulary is used throughout: the unit is an Egg, and its
// lifecycle states are incubating, hatching, hatched, cold, and cracked.
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Nocturnal ember palette.
const (
	ColourEmber     = lipgloss.Color("#ff7a2f") // primary ember
	ColourEmberSoft = lipgloss.Color("#ffab5e") // soft ember highlight
	ColourBone      = lipgloss.Color("#f4ede1") // bone, primary text
	ColourMuted     = lipgloss.Color("#a2917a") // muted, secondary text
	ColourLive      = lipgloss.Color("#7fe0a8") // live/green, healthy
	ColourWarn      = lipgloss.Color("#ffcf5e") // warn/amber, in progress
	ColourBad       = lipgloss.Color("#ff6b57") // bad/red, failure
	ColourDimBg     = lipgloss.Color("#1a1512") // dim background
)

// Shared text styles.
var (
	// Title is a bold ember heading.
	Title = lipgloss.NewStyle().Foreground(ColourEmber).Bold(true)

	// Subtle is muted secondary text.
	Subtle = lipgloss.NewStyle().Foreground(ColourMuted)

	// Ember highlights a value in the ember colour.
	Ember = lipgloss.NewStyle().Foreground(ColourEmber)

	// Bone is the primary body text style.
	Bone = lipgloss.NewStyle().Foreground(ColourBone)

	// Key styles a label or key in a soft ember.
	Key = lipgloss.NewStyle().Foreground(ColourEmberSoft).Bold(true)
)

// StateStyle returns the lipgloss style for an Egg lifecycle state.
//
//	hatched              -> live/green
//	incubating           -> amber (building)
//	hatching             -> ember (deploying)
//	cold                 -> muted (stopped or suspended)
//	cracked              -> red (crashed)
func StateStyle(state string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "hatched":
		return lipgloss.NewStyle().Foreground(ColourLive).Bold(true)
	case "incubating":
		return lipgloss.NewStyle().Foreground(ColourWarn).Bold(true)
	case "hatching":
		return lipgloss.NewStyle().Foreground(ColourEmber).Bold(true)
	case "cold":
		return lipgloss.NewStyle().Foreground(ColourMuted)
	case "cracked":
		return lipgloss.NewStyle().Foreground(ColourBad).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(ColourMuted)
	}
}

// StateLabel returns a coloured, human-readable label for an Egg lifecycle
// state. Unknown states are rendered muted and title-cased.
func StateLabel(state string) string {
	normalised := strings.ToLower(strings.TrimSpace(state))
	label := stateLabelText(normalised)
	return StateStyle(normalised).Render(label)
}

// stateLabelText maps a state to its display word, pairing the machine word
// with a short human hint.
func stateLabelText(state string) string {
	switch state {
	case "hatched":
		return "hatched"
	case "incubating":
		return "incubating"
	case "hatching":
		return "hatching"
	case "cold":
		return "cold"
	case "cracked":
		return "cracked"
	case "":
		return "unknown"
	default:
		return state
	}
}

// Logo returns the small ember egg mark followed by the ruust wordmark, ready
// to print. The egg glyph is drawn in ember; "ruust" in bone.
func Logo() string {
	egg := lipgloss.NewStyle().Foreground(ColourEmber).Render("\U0001F95A") // egg glyph
	word := lipgloss.NewStyle().Foreground(ColourBone).Bold(true).Render("ruust")
	return egg + " " + word
}

// ResourceBar renders a small terminal-safe bar for a value out of a maximum,
// filled in ember and padded with a dim track, followed by an optional label.
//
// width is the number of cells for the bar. A width of zero renders just the
// label.
func ResourceBar(value, max float64, width int, label string) string {
	if width <= 0 {
		if label == "" {
			return ""
		}
		return Subtle.Render(label)
	}
	if max <= 0 {
		max = 1
	}
	ratio := value / max
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	filled := int(ratio*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	empty := width - filled

	fill := lipgloss.NewStyle().Foreground(ColourEmber).Render(strings.Repeat("█", filled))
	track := lipgloss.NewStyle().Foreground(ColourMuted).Render(strings.Repeat("░", empty))

	bar := fill + track
	if label != "" {
		bar += " " + Subtle.Render(label)
	}
	return bar
}
