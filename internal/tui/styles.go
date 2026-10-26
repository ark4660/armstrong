package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type styles struct {
	title, tabActive, tabInactive, rule lipgloss.Style
	label, value, muted, key, help      lipgloss.Style
	ok, warn, err, accent, button       lipgloss.Style
	logWarn, logErr, logMuted           lipgloss.Style
	logo                                []color.Color // wordmark gradient, top to bottom
}

// newStyles picks colors for a dark or light terminal background.
func newStyles(dark bool) styles {
	ld := lipgloss.LightDark(dark)
	accent := ld(lipgloss.Color("#0b7a75"), lipgloss.Color("#4fd1c5"))
	muted := ld(lipgloss.Color("#6b7280"), lipgloss.Color("#8b949e"))
	fg := ld(lipgloss.Color("#1f2328"), lipgloss.Color("#e6edf3"))
	warn := ld(lipgloss.Color("#9a6700"), lipgloss.Color("#e3b341"))
	errC := ld(lipgloss.Color("#cf222e"), lipgloss.Color("#ff7b72"))
	ok := ld(lipgloss.Color("#1a7f37"), lipgloss.Color("#3fb950"))

	return styles{
		title:       lipgloss.NewStyle().Bold(true).Foreground(accent),
		tabActive:   lipgloss.NewStyle().Bold(true).Foreground(fg).Underline(true).Padding(0, 1),
		tabInactive: lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		rule:        lipgloss.NewStyle().Foreground(muted),
		label:       lipgloss.NewStyle().Foreground(muted).Width(12),
		value:       lipgloss.NewStyle().Foreground(fg),
		muted:       lipgloss.NewStyle().Foreground(muted),
		key:         lipgloss.NewStyle().Foreground(accent).Bold(true),
		help:        lipgloss.NewStyle().Foreground(muted),
		ok:          lipgloss.NewStyle().Foreground(ok),
		warn:        lipgloss.NewStyle().Foreground(warn),
		err:         lipgloss.NewStyle().Foreground(errC),
		accent:      lipgloss.NewStyle().Foreground(accent),
		button:      lipgloss.NewStyle().Bold(true).Foreground(accent).Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 3),
		logWarn:     lipgloss.NewStyle().Foreground(warn),
		logErr:      lipgloss.NewStyle().Foreground(errC),
		logMuted:    lipgloss.NewStyle().Foreground(muted),
		logo:        logoGradient(dark),
	}
}
