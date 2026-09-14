package vt

import (
	"slices"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// PhysicalLine is an immutable snapshot of one terminal row.
type PhysicalLine struct {
	cells     uv.Line
	wrapped   bool
	wrapWidth int
}

// Cells returns a copy of the cells in the physical line.
func (l PhysicalLine) Cells() uv.Line {
	return slices.Clone(l.cells)
}

// String returns the plain-text contents of the physical line. It preserves
// spaces consumed before an automatic wrap while omitting unused right-side
// padding. Wide-cell placeholders are not included.
func (l PhysicalLine) String() string {
	if l.wrapWidth > 0 {
		var text strings.Builder
		for _, cell := range l.cells[:min(l.wrapWidth, len(l.cells))] {
			if !cell.IsZero() {
				text.WriteString(cell.Content)
			}
		}
		return text.String()
	}
	return l.cells.String()
}

// Render returns the physical line with styles and links encoded as ANSI
// escape sequences.
func (l PhysicalLine) Render() string {
	return l.cells.Render()
}

// Wrapped reports whether this physical line continues onto the next line in
// the same snapshot because of automatic wrapping. A false value marks a hard
// line boundary, including boundaries created by CR/LF. The last line in a
// snapshot is always reported as not wrapped.
func (l PhysicalLine) Wrapped() bool {
	return l.wrapped
}

func newPhysicalLine(line uv.Line, wrapped bool, wrapWidth int) PhysicalLine {
	return PhysicalLine{
		cells:     slices.Clone(line),
		wrapped:   wrapped,
		wrapWidth: wrapWidth,
	}
}

// PhysicalLines returns an immutable snapshot containing the main screen's
// scrollback followed by every row of the active screen, ordered oldest to
// newest. When the alternate screen is active, the boundary between main
// scrollback and the alternate screen is always a hard line boundary.
func (e *Emulator) PhysicalLines() []PhysicalLine {
	scrollback := e.scrs[0].Scrollback()
	scrollbackLen := 0
	if scrollback != nil {
		scrollbackLen = scrollback.Len()
	}

	lines := make([]PhysicalLine, 0, scrollbackLen+e.scr.Height())
	for i := 0; i < scrollbackLen; i++ {
		lines = append(lines, newPhysicalLine(scrollback.lines[i], scrollback.wrapped[i], scrollback.wrapWidth[i]))
	}
	if e.IsAltScreen() && len(lines) > 0 {
		lines[len(lines)-1].wrapped = false
	}

	for y := 0; y < e.scr.Height(); y++ {
		wrapped := y < len(e.scr.wrapped) && e.scr.wrapped[y]
		wrapWidth := e.scr.wrapWidth[y]
		if e.atPhantom && y == e.scr.cur.Y {
			wrapWidth = e.scr.Width()
		}
		lines = append(lines, newPhysicalLine(e.scr.buf.Line(y), wrapped, wrapWidth))
	}
	if len(lines) > 0 {
		lines[len(lines)-1].wrapped = false
	}
	return lines
}
