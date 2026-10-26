package tui

import (
	"strings"
	"unicode/utf8"
)

// picker is a scrollable list with a cursor and, if filterable, a filter
// that typing narrows. Rendering is up to the caller.
type picker struct {
	items      []string
	cursor     int // index into the filtered view
	offset     int // first visible row
	height     int
	filterable bool
	filter     string
}

func (p *picker) setItems(items []string) {
	p.items = items
	p.clamp()
}

// matches returns the indexes of items that pass the filter.
func (p *picker) matches() []int {
	var out []int
	f := strings.ToLower(p.filter)
	for i, it := range p.items {
		if f == "" || strings.Contains(strings.ToLower(it), f) {
			out = append(out, i)
		}
	}
	return out
}

// selected returns the item index under the cursor, or -1.
func (p *picker) selected() int {
	m := p.matches()
	if p.cursor < 0 || p.cursor >= len(m) {
		return -1
	}
	return m[p.cursor]
}

// selectIndex moves the cursor to item i (clearing the filter).
func (p *picker) selectIndex(i int) {
	p.filter = ""
	p.cursor = i
	p.clamp()
}

func (p *picker) clamp() {
	n := len(p.matches())
	p.cursor = min(max(p.cursor, 0), max(n-1, 0))
	h := max(p.height, 1)
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+h {
		p.offset = p.cursor - h + 1
	}
	p.offset = min(max(p.offset, 0), max(n-h, 0))
}

// handleKey moves the cursor or edits the filter; it reports whether it used
// the key. j/k move the cursor only in lists without a filter.
func (p *picker) handleKey(k string) bool {
	switch {
	case k == "up" || k == "k" && !p.filterable:
		p.cursor--
	case k == "down" || k == "j" && !p.filterable:
		p.cursor++
	case k == "pgup":
		p.cursor -= max(p.height-1, 1)
	case k == "pgdown":
		p.cursor += max(p.height-1, 1)
	case k == "home":
		p.cursor = 0
	case k == "end":
		p.cursor = len(p.items)
	case k == "backspace" && p.filterable && p.filter != "":
		_, size := utf8.DecodeLastRuneInString(p.filter)
		p.filter = p.filter[:len(p.filter)-size]
		p.cursor = 0
	case p.filterable && utf8.RuneCountInString(k) == 1 && k != " ":
		p.filter += k
		p.cursor = 0
	default:
		return false
	}
	p.clamp()
	return true
}

// view renders the visible rows with render(itemIndex, isCursor).
func (p *picker) view(render func(i int, cursor bool) string) []string {
	m := p.matches()
	var rows []string
	for row := p.offset; row < len(m) && row < p.offset+max(p.height, 1); row++ {
		rows = append(rows, render(m[row], row == p.cursor))
	}
	return rows
}
