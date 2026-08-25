package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/ui"
)

// detailView renders the detail pane for the selected Egg: name, state, region,
// tier, url, domains, masked env var keys and the latest deployment.
func (m model) detailView() string {
	header := m.detailHeader()
	footer := m.footer([]key.Binding{
		m.keys.Logs,
		m.keys.Back,
		m.keys.Quit,
	})

	var body string
	if m.detail == nil {
		note := ui.StateStyle("cracked").Render("could not open this Egg")
		if m.detailErr != "" {
			note = lipgloss.JoinVertical(lipgloss.Left, note, "", ui.Bone.Render(m.detailErr))
		}
		body = lipgloss.NewStyle().Padding(2, 3).Render(note)
	} else {
		body = m.detailBody(*m.detail)
	}

	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}
	middle := lipgloss.NewStyle().Width(m.width).Height(contentHeight).Render(body)

	return lipgloss.JoinVertical(lipgloss.Left, header, middle, footer)
}

// detailHeader renders the logo, the Egg name and its coloured state.
func (m model) detailHeader() string {
	title := ui.Logo() + "  " + ui.Subtle.Render("the nest")
	rule := lipgloss.NewStyle().
		Foreground(ui.ColourEmber).
		Render(strings.Repeat("─", max(m.width-4, 1)))
	return lipgloss.NewStyle().Padding(1, 2, 0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, rule),
	)
}

// detailBody composes the Egg detail card.
func (m model) detailBody(d api.EggDetail) string {
	// Title line: mark, name, state.
	titleLine := eggMark + "  " +
		ui.Title.Render(d.Name) + "   " + ui.StateLabel(d.State)

	rows := []string{
		field("region", regionText(d.Egg)),
		field("tier", tierWithPrice(d.Egg)),
		field("url", urlText(d.URL)),
		field("repo", valueOrDash(d.Repo)),
		field("created", valueOrDash(d.CreatedAt)),
	}
	overview := boxSection("overview", lipgloss.JoinVertical(lipgloss.Left, rows...))

	domains := boxSection("domains", m.renderDomains(d.Domains))
	env := boxSection("env keys (masked)", m.renderEnvKeys(d.EnvKeys))
	deploy := boxSection("latest deployment", m.renderDeployment(d.Deployment))

	// Two columns when wide enough, otherwise stacked.
	var layout string
	if m.width >= 90 {
		colW := (m.width - 8) / 2
		leftCol := lipgloss.NewStyle().Width(colW).Render(
			lipgloss.JoinVertical(lipgloss.Left, overview, "", deploy),
		)
		rightCol := lipgloss.NewStyle().Width(colW).Render(
			lipgloss.JoinVertical(lipgloss.Left, domains, "", env),
		)
		layout = lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)
	} else {
		layout = lipgloss.JoinVertical(lipgloss.Left, overview, "", deploy, "", domains, "", env)
	}

	hint := ui.Subtle.Render("press ") + ui.Key.Render("g") +
		ui.Subtle.Render(" to tail the logs")

	return lipgloss.NewStyle().Padding(1, 3).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleLine, "", layout, "", hint),
	)
}

// field renders a "key  value" line with an aligned ember key.
func field(k, v string) string {
	label := lipgloss.NewStyle().
		Foreground(ui.ColourEmberSoft).
		Bold(true).
		Width(10).
		Render(k)
	return label + " " + ui.Bone.Render(v)
}

// boxSection wraps content in a titled ember-bordered box.
func boxSection(title, content string) string {
	heading := ui.Key.Render(title)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColourEmber).
		Padding(0, 1).
		Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, heading, box)
}

// renderDomains lists the hostnames attached to the Egg, marking custom ones and
// showing their cert status.
func (m model) renderDomains(domains []api.Domain) string {
	if len(domains) == 0 {
		return ui.Subtle.Render("no domains yet")
	}
	lines := make([]string, 0, len(domains))
	for _, d := range domains {
		kind := ui.Subtle.Render("default")
		if d.IsCustom {
			kind = ui.Ember.Render("custom")
		}
		host := ui.Bone.Render(d.Hostname)
		cert := ui.Subtle.Render(certText(d.CertStatus))
		lines = append(lines, fmt.Sprintf("%s  %s  %s", host, kind, cert))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// certText renders a cert status, defaulting to a dash when empty.
func certText(status string) string {
	if strings.TrimSpace(status) == "" {
		return "cert: -"
	}
	return "cert: " + status
}

// renderEnvKeys lists env var keys with their values masked. Only the KEYS are
// ever shown, never any value.
func (m model) renderEnvKeys(keys []string) string {
	if len(keys) == 0 {
		return ui.Subtle.Render("no env vars set")
	}
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines,
			ui.Bone.Render(k)+ui.Subtle.Render(" = ")+ui.Subtle.Render("••••••••"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderDeployment renders the latest deployment, or a note when there is none.
func (m model) renderDeployment(dep *api.Deployment) string {
	if dep == nil {
		return ui.Subtle.Render("no deployments yet")
	}
	rows := []string{
		field("status", deployStatus(dep.Status)),
		field("commit", shortSha(dep.GitSha)),
		field("image", ptrOrDash(dep.ImageRef)),
		field("finished", ptrOrDash(dep.FinishedAt)),
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// deployStatus colours a deployment status word in the ember vocabulary, reusing
// the lifecycle palette where the words overlap.
func deployStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "succeeded", "success", "ok", "done", "hatched":
		return lipgloss.NewStyle().Foreground(ui.ColourLive).Bold(true).Render(status)
	case "failed", "error", "cracked":
		return lipgloss.NewStyle().Foreground(ui.ColourBad).Bold(true).Render(status)
	case "building", "running", "pending", "incubating", "hatching", "in_progress":
		return lipgloss.NewStyle().Foreground(ui.ColourWarn).Bold(true).Render(status)
	case "":
		return ui.Subtle.Render("-")
	default:
		return ui.Bone.Render(status)
	}
}

// tierWithPrice renders the tier label alongside its flat GBP monthly price.
func tierWithPrice(e api.Egg) string {
	tier := tierText(e)
	if e.PriceGbp > 0 {
		return fmt.Sprintf("%s  £%g/mo unmetered egress", tier, e.PriceGbp)
	}
	return tier
}

// urlText renders the host with an implied https scheme, or a dash when empty.
func urlText(host string) string {
	if strings.TrimSpace(host) == "" {
		return "-"
	}
	return "https://" + host
}

// shortSha trims a git sha to a readable prefix.
func shortSha(sha string) string {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return "-"
	}
	if len(sha) > 10 {
		return sha[:10]
	}
	return sha
}

// valueOrDash returns s, or a dash when s is blank.
func valueOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// ptrOrDash returns *s, or a dash when the pointer is nil or blank.
func ptrOrDash(s *string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return "-"
	}
	return *s
}

// logsView renders the scrollable log viewport for the current Egg.
func (m model) logsView() string {
	header := m.logsHeader()
	footer := m.footer([]key.Binding{
		m.keys.Up,
		m.keys.Down,
		m.keys.Back,
		m.keys.Quit,
	})

	var pane string
	if m.logsErr != "" {
		note := ui.StateStyle("cracked").Render("could not read the logs") + "\n\n" +
			ui.Bone.Render(m.logsErr)
		pane = lipgloss.NewStyle().Padding(1, 2).Render(note)
	} else if m.vpReady {
		pane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColourEmber).
			Render(m.viewport.View())
	}

	scroll := ""
	if m.vpReady && m.logsErr == "" {
		pct := int(m.viewport.ScrollPercent() * 100)
		scroll = ui.Subtle.Render(fmt.Sprintf("  %d%%", pct))
	}
	paneRow := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Padding(0, 2).Render(pane),
		lipgloss.NewStyle().Padding(0, 2).Render(scroll),
	)

	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}
	middle := lipgloss.NewStyle().Width(m.width).Height(contentHeight).Render(paneRow)

	return lipgloss.JoinVertical(lipgloss.Left, header, middle, footer)
}

// logsHeader shows the logo and which Egg's logs are being tailed.
func (m model) logsHeader() string {
	name := "logs"
	if m.detail != nil {
		name = "logs · " + m.detail.Name
	}
	title := ui.Logo() + "  " + ui.Subtle.Render(name)
	rule := lipgloss.NewStyle().
		Foreground(ui.ColourEmber).
		Render(strings.Repeat("─", max(m.width-4, 1)))
	return lipgloss.NewStyle().Padding(1, 2, 0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, rule),
	)
}

// renderLogBody formats the log lines for the viewport, colouring the level and
// dimming the timestamp.
func (m model) renderLogBody() string {
	if len(m.logs) == 0 {
		return ui.Subtle.Render("no log lines yet")
	}
	lines := make([]string, 0, len(m.logs))
	for _, l := range m.logs {
		ts := ui.Subtle.Render(l.TS)
		lvl := logLevel(l.Level)
		lines = append(lines, fmt.Sprintf("%s %s %s", ts, lvl, ui.Bone.Render(l.Text)))
	}
	return strings.Join(lines, "\n")
}

// logLevel colours a log level word and pads it to a fixed width.
func logLevel(level string) string {
	raw := strings.ToUpper(strings.TrimSpace(level))
	if raw == "" {
		raw = "LOG"
	}
	padded := fmt.Sprintf("%-5s", raw)
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error", "fatal", "err":
		return lipgloss.NewStyle().Foreground(ui.ColourBad).Bold(true).Render(padded)
	case "warn", "warning":
		return lipgloss.NewStyle().Foreground(ui.ColourWarn).Bold(true).Render(padded)
	case "info":
		return lipgloss.NewStyle().Foreground(ui.ColourLive).Render(padded)
	case "debug", "trace":
		return ui.Subtle.Render(padded)
	default:
		return ui.Ember.Render(padded)
	}
}
