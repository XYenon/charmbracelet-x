package vt

import "strings"

// LogicalLine is an immutable plain-text snapshot of one terminal logical
// line. Physical rows joined by automatic wrapping belong to the same logical
// line; hard line boundaries, including CR/LF, start a new one.
type LogicalLine struct {
	text string
}

// String returns the plain-text contents of the logical line.
func (l LogicalLine) String() string {
	return l.text
}

// LogicalLines returns an immutable snapshot containing the main screen's
// scrollback followed by the active screen, grouped into logical lines.
// Automatic soft wraps are joined while hard line boundaries remain separate.
func (e *Emulator) LogicalLines() []LogicalLine {
	physical := e.PhysicalLines()
	lines := make([]LogicalLine, 0, len(physical))
	var text strings.Builder
	for _, line := range physical {
		text.WriteString(line.String())
		if !line.Wrapped() {
			lines = append(lines, LogicalLine{text: text.String()})
			text.Reset()
		}
	}
	return lines
}
