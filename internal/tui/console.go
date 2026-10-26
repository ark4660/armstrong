package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// maxLogLines caps memory use; the game's full log is in logs/latest.log.
const maxLogLines = 5000

// console shows the game's output. It follows new output while scrolled to
// the bottom; scrolling up pauses following until f or G.
type console struct {
	vp     viewport.Model
	raw    []string // as the game printed them, for copying
	styled []string
	follow bool
}

func newConsole() console {
	vp := viewport.New()
	vp.SoftWrap = true // stack traces are long; wrapping beats horizontal scrolling
	return console{vp: vp, follow: true}
}

func (c *console) resize(w, h int) {
	c.vp.SetWidth(max(w, 1))
	c.vp.SetHeight(max(h, 1))
	if c.follow {
		c.vp.GotoBottom()
	}
}

func (c *console) clear() {
	c.raw, c.styled = nil, nil
	c.follow = true
	c.vp.SetContentLines(nil)
}

func (c *console) append(s styles, lines []string) {
	for _, l := range lines {
		l = strings.ReplaceAll(l, "\t", "    ")
		c.raw = append(c.raw, l)
		c.styled = append(c.styled, styleLine(s, l))
	}
	if over := len(c.raw) - maxLogLines; over > 0 {
		c.raw = c.raw[over:]
		c.styled = c.styled[over:]
	}
	c.vp.SetContentLines(c.styled)
	if c.follow {
		c.vp.GotoBottom()
	}
}

// restyle re-renders every line after the theme changes.
func (c *console) restyle(s styles) {
	for i, l := range c.raw {
		c.styled[i] = styleLine(s, l)
	}
	c.vp.SetContentLines(c.styled)
}

func styleLine(s styles, l string) string {
	switch {
	case strings.Contains(l, "/ERROR]") || strings.Contains(l, "/FATAL]") || strings.HasPrefix(l, "Exception") || strings.HasPrefix(l, "Caused by"):
		return s.logErr.Render(l)
	case strings.Contains(l, "/WARN]"):
		return s.logWarn.Render(l)
	case strings.HasPrefix(l, "    at ") || strings.HasPrefix(l, "── "):
		return s.logMuted.Render(l)
	}
	return l
}

func (c *console) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "f":
		c.follow = !c.follow
		if c.follow {
			c.vp.GotoBottom()
		}
		return nil
	case "G", "end":
		c.follow = true
		c.vp.GotoBottom()
		return nil
	case "g", "home":
		c.follow = false
		c.vp.GotoTop()
		return nil
	case "y":
		// OSC 52 payloads are limited in many terminals; copy the recent tail.
		return tea.SetClipboard(strings.Join(c.raw[max(len(c.raw)-500, 0):], "\n"))
	}
	var cmd tea.Cmd
	c.vp, cmd = c.vp.Update(msg)
	c.follow = c.vp.AtBottom()
	return cmd
}

func (c console) view() string {
	if len(c.raw) == 0 {
		return "\n  No game output yet. Press tab, then enter to play."
	}
	return c.vp.View()
}
