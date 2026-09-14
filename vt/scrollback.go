package vt

import (
	"slices"

	uv "github.com/charmbracelet/ultraviolet"
)

// DefaultScrollbackSize is the default maximum number of lines in the scrollback buffer.
const DefaultScrollbackSize = 10000

// Scrollback represents a scrollback buffer that stores lines scrolled off the screen.
type Scrollback struct {
	lines                 []uv.Line
	wrapped               []bool
	wrapWidth             []int
	maxLines              int
	maxLogicalLines       int
	headPartial           bool
	discardingLogicalLine bool
}

// NewScrollback creates a new scrollback buffer with the given maximum number of lines.
func NewScrollback(maxLines int) *Scrollback {
	if maxLines <= 0 {
		maxLines = DefaultScrollbackSize
	}
	return &Scrollback{
		lines:    make([]uv.Line, 0, min(maxLines, 1000)), // Pre-allocate reasonable capacity
		maxLines: maxLines,
	}
}

// Push adds a line to the scrollback buffer.
// If the buffer is full, the oldest line is removed.
func (s *Scrollback) Push(line uv.Line) {
	s.push(line, false, 0)
}

// push adds a line and records whether it continues onto the next line.
func (s *Scrollback) push(line uv.Line, wrapped bool, wrapWidth int) {
	if s == nil || s.maxLines <= 0 {
		return
	}
	if s.discardingLogicalLine {
		if !wrapped {
			s.discardingLogicalLine = false
		}
		return
	}

	last := len(line)
	if !wrapped && wrapWidth <= 0 {
		// Trailing empty cells on a hard line are not part of its contents.
		last = 0
		for i := len(line) - 1; i >= 0; i-- {
			c := &line[i]
			if !c.IsZero() && !c.Equal(&uv.EmptyCell) {
				last = i + 1
				break
			}
		}
	}

	cloned := slices.Clone(line[:last])

	if s.maxLogicalLines <= 0 && len(s.lines) >= s.maxLines {
		s.headPartial = s.wrapped[0]
		s.lines = slices.Delete(s.lines, 0, 1)
		s.wrapped = slices.Delete(s.wrapped, 0, 1)
		s.wrapWidth = slices.Delete(s.wrapWidth, 0, 1)
	}
	s.lines = append(s.lines, cloned)
	s.wrapped = append(s.wrapped, wrapped)
	s.wrapWidth = append(s.wrapWidth, wrapWidth)
	if s.maxLogicalLines > 0 {
		s.trimLogicalLines()
	}
}

// PushN adds n lines from the buffer starting at line y to the scrollback.
func (s *Scrollback) PushN(buf *uv.RenderBuffer, y, n int) {
	if s == nil || buf == nil || n <= 0 {
		return
	}

	for i := range min(n, buf.Height()-y) {
		if line := buf.Line(y + i); line != nil {
			s.Push(line)
		}
	}
}

// Len returns the number of lines in the scrollback buffer.
func (s *Scrollback) Len() int {
	if s == nil {
		return 0
	}
	return len(s.lines)
}

// MaxLines returns the maximum number of lines the scrollback buffer can hold.
func (s *Scrollback) MaxLines() int {
	if s == nil {
		return 0
	}
	return s.maxLines
}

// SetMaxLines sets the maximum number of lines in the scrollback buffer.
// If the current number of lines exceeds the new maximum, oldest lines are removed.
func (s *Scrollback) SetMaxLines(maxLines int) {
	if s == nil || maxLines <= 0 {
		return
	}

	s.maxLines = maxLines
	if len(s.lines) > maxLines {
		if s.maxLogicalLines > 0 {
			s.trimLogicalLines()
		} else {
			cut := len(s.lines) - maxLines
			s.headPartial = s.wrapped[cut-1]
			s.lines = s.lines[cut:]
			s.wrapped = s.wrapped[cut:]
			s.wrapWidth = s.wrapWidth[cut:]
		}
	}
}

// SetMaxLogicalLines sets the maximum number of logical lines retained in
// scrollback. Soft-wrapped physical rows are kept or evicted as a group. The
// physical row limit set by [Scrollback.SetMaxLines] remains a hard memory
// bound; a logical line larger than that limit is discarded in full.
func (s *Scrollback) SetMaxLogicalLines(maxLines int) {
	if s == nil || maxLines <= 0 {
		return
	}

	s.maxLogicalLines = maxLines
	s.trimLogicalLines()
}

// Line returns the line at the given index.
// Index 0 is the oldest line, Len()-1 is the most recent.
// Returns nil if index is out of bounds.
func (s *Scrollback) Line(index int) uv.Line {
	if s == nil || index < 0 || index >= len(s.lines) {
		return nil
	}
	return s.lines[index]
}

// Lines returns all lines in the scrollback buffer.
// Index 0 is the oldest line.
func (s *Scrollback) Lines() []uv.Line {
	if s == nil {
		return nil
	}
	return s.lines
}

// Clear removes all lines from the scrollback buffer.
func (s *Scrollback) Clear() {
	if s == nil {
		return
	}
	s.lines = s.lines[:0]
	s.wrapped = s.wrapped[:0]
	s.wrapWidth = s.wrapWidth[:0]
	s.headPartial = false
	s.discardingLogicalLine = false
}

// CellAt returns the cell at the given position in the scrollback buffer.
// x is the column, y is the line index (0 = oldest).
// Returns nil if position is out of bounds.
func (s *Scrollback) CellAt(x, y int) *uv.Cell {
	line := s.Line(y)
	if line == nil || x < 0 || x >= len(line) {
		return nil
	}
	return &line[x]
}

func (s *Scrollback) trimLogicalLines() {
	if s.headPartial {
		s.dropPartialHead()
	}
	for s.logicalLineCount() > s.maxLogicalLines {
		if !s.dropOldestLogicalLine() {
			break
		}
	}
	for len(s.lines) > s.maxLines {
		if !s.dropOldestLogicalLine() {
			s.discardingLogicalLine = s.wrapped[len(s.wrapped)-1]
			s.lines = s.lines[:0]
			s.wrapped = s.wrapped[:0]
			s.wrapWidth = s.wrapWidth[:0]
			break
		}
	}
}

func (s *Scrollback) logicalLineCount() int {
	if len(s.lines) == 0 {
		return 0
	}

	count := 0
	for _, wrapped := range s.wrapped {
		if !wrapped {
			count++
		}
	}
	if s.wrapped[len(s.wrapped)-1] {
		count++
	}
	return count
}

func (s *Scrollback) dropOldestLogicalLine() bool {
	for i, wrapped := range s.wrapped {
		if !wrapped {
			s.lines = slices.Delete(s.lines, 0, i+1)
			s.wrapped = slices.Delete(s.wrapped, 0, i+1)
			s.wrapWidth = slices.Delete(s.wrapWidth, 0, i+1)
			return true
		}
	}
	return false
}

func (s *Scrollback) dropPartialHead() {
	s.headPartial = false
	if s.dropOldestLogicalLine() {
		return
	}
	if len(s.wrapped) > 0 {
		s.discardingLogicalLine = s.wrapped[len(s.wrapped)-1]
	}
	s.lines = s.lines[:0]
	s.wrapped = s.wrapped[:0]
	s.wrapWidth = s.wrapWidth[:0]
}
