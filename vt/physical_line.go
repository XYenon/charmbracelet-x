package vt

import (
	"slices"

	uv "github.com/charmbracelet/ultraviolet"
)

// PhysicalLine is an immutable snapshot of one terminal row.
type PhysicalLine struct {
	cells   uv.Line
	wrapped bool
}

// Cells returns a copy of the cells in the physical line.
func (l PhysicalLine) Cells() uv.Line {
	return slices.Clone(l.cells)
}

// String returns the plain-text contents of the physical line.
func (l PhysicalLine) String() string {
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

func newPhysicalLine(line uv.Line, wrapped bool) PhysicalLine {
	return PhysicalLine{
		cells:   slices.Clone(line),
		wrapped: wrapped,
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
		lines = append(lines, newPhysicalLine(scrollback.lines[i], scrollback.wrapped[i]))
	}
	if e.IsAltScreen() && len(lines) > 0 {
		lines[len(lines)-1].wrapped = false
	}

	for y := 0; y < e.scr.Height(); y++ {
		wrapped := y < len(e.scr.wrapped) && e.scr.wrapped[y]
		lines = append(lines, newPhysicalLine(e.scr.buf.Line(y), wrapped))
	}
	if len(lines) > 0 {
		lines[len(lines)-1].wrapped = false
	}
	return lines
}
