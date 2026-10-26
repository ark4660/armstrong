package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/ark4660/armstrong/internal/app"
)

// The ARMSTRONG wordmark in two sizes ("ANSI Shadow" style, and a two-row
// block version for narrow terminals). Every row of a size has the same width.
var (
	logoBig = []string{
		` █████╗ ██████╗ ███╗   ███╗███████╗████████╗██████╗  ██████╗ ███╗   ██╗ ██████╗ `,
		`██╔══██╗██╔══██╗████╗ ████║██╔════╝╚══██╔══╝██╔══██╗██╔═══██╗████╗  ██║██╔════╝ `,
		`███████║██████╔╝██╔████╔██║███████╗   ██║   ██████╔╝██║   ██║██╔██╗ ██║██║  ███╗`,
		`██╔══██║██╔══██╗██║╚██╔╝██║╚════██║   ██║   ██╔══██╗██║   ██║██║╚██╗██║██║   ██║`,
		`██║  ██║██║  ██║██║ ╚═╝ ██║███████║   ██║   ██║  ██║╚██████╔╝██║ ╚████║╚██████╔╝`,
		`╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝ `,
	}
	logoSmall = []string{
		`▄▀█ █▀█ █▀▄▀█ █▀ ▀█▀ █▀█ █▀█ █▄ █ █▀▀`,
		`█▀█ █▀▄ █ ▀ █ ▄█  █  █▀▄ █▄█ █ ▀█ █▄█`,
	}
)

// logoView renders the biggest wordmark that fits in width × height, with a
// top-to-bottom gradient. It falls back to plain text on tiny terminals.
func (m model) logoView(width, height int) string {
	rows := logoBig
	switch {
	case width >= lipgloss.Width(logoBig[0])+4 && height >= 34:
	case width >= lipgloss.Width(logoSmall[0])+4:
		rows = logoSmall
	default:
		return m.styles.title.Render(app.DisplayName)
	}
	colors := m.styles.logo
	out := make([]string, len(rows))
	for i, r := range rows {
		c := colors[i*(len(colors)-1)/max(len(rows)-1, 1)]
		out[i] = lipgloss.NewStyle().Foreground(c).Bold(true).Render(strings.TrimRight(r, " "))
	}
	return strings.Join(out, "\n")
}

// logoGradient goes from the accent teal to a deep blue.
func logoGradient(dark bool) []color.Color {
	if dark {
		return []color.Color{
			lipgloss.Color("#5eead4"), lipgloss.Color("#4fd1c5"), lipgloss.Color("#38bdf8"),
			lipgloss.Color("#3b82f6"), lipgloss.Color("#6366f1"), lipgloss.Color("#7c3aed"),
		}
	}
	return []color.Color{
		lipgloss.Color("#0f766e"), lipgloss.Color("#0b7a75"), lipgloss.Color("#0369a1"),
		lipgloss.Color("#1d4ed8"), lipgloss.Color("#4338ca"), lipgloss.Color("#6d28d9"),
	}
}
