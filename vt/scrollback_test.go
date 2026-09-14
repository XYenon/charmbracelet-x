package vt

import (
	"slices"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestScrollback(t *testing.T) {
	t.Run("basic push and len", func(t *testing.T) {
		sb := NewScrollback(100)
		if sb.Len() != 0 {
			t.Errorf("expected len 0, got %d", sb.Len())
		}
		if sb.MaxLines() != 100 {
			t.Errorf("expected max 100, got %d", sb.MaxLines())
		}
	})

	t.Run("scrollback in emulator", func(t *testing.T) {
		// Create a small terminal
		e := NewEmulator(10, 5)

		// Fill the screen with numbered lines and force scrolling
		for i := 0; i < 10; i++ {
			e.WriteString("\r\n") // Scroll up
		}

		// Check scrollback has captured some lines
		sbLen := e.ScrollbackLen()
		t.Logf("scrollback length after 10 newlines: %d", sbLen)

		if sbLen == 0 {
			t.Error("expected scrollback to have captured lines, got 0")
		}
	})

	t.Run("scrollback with content", func(t *testing.T) {
		e := NewEmulator(20, 5)

		// Write content that will scroll
		for i := 0; i < 10; i++ {
			e.WriteString("line\r\n")
		}

		// Verify scrollback captured the scrolled content
		sb := e.Scrollback()
		if sb == nil {
			t.Fatal("scrollback is nil")
		}

		// Should have captured lines (at least 5, since screen is 5 tall and we wrote 10 lines)
		if sb.Len() < 5 {
			t.Errorf("expected at least 5 lines in scrollback, got %d", sb.Len())
		}
	})

	t.Run("scrollback max lines", func(t *testing.T) {
		sb := NewScrollback(5)

		// Push more lines than max
		for i := 0; i < 10; i++ {
			sb.Push(nil)
		}

		if sb.Len() != 5 {
			t.Errorf("expected len 5 after overflow, got %d", sb.Len())
		}
	})

	t.Run("clear scrollback", func(t *testing.T) {
		e := NewEmulator(20, 5)

		// Write content that will scroll
		for i := 0; i < 10; i++ {
			e.WriteString("line\r\n")
		}

		// Verify we have scrollback
		if e.ScrollbackLen() == 0 {
			t.Error("expected scrollback before clear")
		}

		// Clear it
		e.ClearScrollback()

		if e.ScrollbackLen() != 0 {
			t.Errorf("expected empty scrollback after clear, got %d", e.ScrollbackLen())
		}
	})

	t.Run("alt screen does not have scrollback", func(t *testing.T) {
		e := NewEmulator(20, 5)

		// Write some content to main screen
		for i := 0; i < 10; i++ {
			e.WriteString("line\r\n")
		}

		mainScrollbackLen := e.ScrollbackLen()
		if mainScrollbackLen == 0 {
			t.Error("expected scrollback on main screen")
		}

		// Enter alt screen
		e.WriteString("\x1b[?1049h") // DECSET alt screen

		// Scrollback should still be from main screen
		if e.ScrollbackLen() != mainScrollbackLen {
			t.Errorf("expected scrollback len %d in alt screen, got %d",
				mainScrollbackLen, e.ScrollbackLen())
		}

		// Write to alt screen - should not affect main scrollback
		for i := 0; i < 10; i++ {
			e.WriteString("alt\r\n")
		}

		// Main screen scrollback should be unchanged
		if e.ScrollbackLen() != mainScrollbackLen {
			t.Errorf("expected scrollback len %d after alt screen writes, got %d",
				mainScrollbackLen, e.ScrollbackLen())
		}
	})

	t.Run("ED 2 saves to scrollback", func(t *testing.T) {
		e := NewEmulator(20, 5)

		// Write some content (not enough to scroll)
		e.WriteString("line 1\r\n")
		e.WriteString("line 2\r\n")
		e.WriteString("line 3\r\n")

		// Should have no scrollback yet (didn't scroll)
		initialLen := e.ScrollbackLen()

		// Clear screen with ED 2 (ESC[2J)
		e.WriteString("\x1b[2J")

		// Should have saved lines to scrollback
		newLen := e.ScrollbackLen()
		if newLen <= initialLen {
			t.Errorf("expected scrollback to grow after ED 2, was %d now %d", initialLen, newLen)
		}
		t.Logf("scrollback after ED 2: %d lines", newLen)
	})

	t.Run("ED 3 clears scrollback", func(t *testing.T) {
		e := NewEmulator(20, 5)

		// Write content that will scroll
		for i := 0; i < 10; i++ {
			e.WriteString("line\r\n")
		}

		// Verify we have scrollback
		if e.ScrollbackLen() == 0 {
			t.Error("expected scrollback before ED 3")
		}

		// ED 3 (ESC[3J) should clear scrollback
		e.WriteString("\x1b[3J")

		if e.ScrollbackLen() != 0 {
			t.Errorf("expected empty scrollback after ED 3, got %d", e.ScrollbackLen())
		}
	})
}

func TestLogicalScrollbackLimitKeepsCompleteLines(t *testing.T) {
	sb := NewScrollback(100)
	sb.SetMaxLogicalLines(2)

	for _, line := range []struct {
		text    string
		wrapped bool
	}{
		{"aaaa", true}, {"bbbb", false},
		{"cccc", true}, {"dddd", false},
		{"eeee", true}, {"ffff", false},
	} {
		sb.push(lineFromString(line.text), line.wrapped, 4)
	}

	if got, want := scrollbackStrings(sb), []string{"cccc", "dddd", "eeee", "ffff"}; !slices.Equal(got, want) {
		t.Fatalf("physical lines = %q, want %q", got, want)
	}
	if got, want := sb.wrapped, []bool{true, false, true, false}; !slices.Equal(got, want) {
		t.Fatalf("wrapped flags = %v, want %v", got, want)
	}
}

func TestLogicalScrollbackPhysicalLimitKeepsCompleteLines(t *testing.T) {
	sb := NewScrollback(3)
	sb.SetMaxLogicalLines(10)

	for _, line := range []struct {
		text    string
		wrapped bool
	}{
		{"aaaa", true}, {"bbbb", false},
		{"cccc", true}, {"dddd", false},
	} {
		sb.push(lineFromString(line.text), line.wrapped, 4)
	}

	if got, want := scrollbackStrings(sb), []string{"cccc", "dddd"}; !slices.Equal(got, want) {
		t.Fatalf("physical lines = %q, want %q", got, want)
	}
}

func TestLogicalScrollbackDiscardsOversizedLine(t *testing.T) {
	sb := NewScrollback(2)
	sb.SetMaxLogicalLines(10)

	sb.push(lineFromString("aaaa"), true, 4)
	sb.push(lineFromString("bbbb"), true, 4)
	sb.push(lineFromString("cccc"), true, 4)
	sb.push(lineFromString("dddd"), false, 4)
	sb.push(lineFromString("next"), false, 4)

	if got, want := scrollbackStrings(sb), []string{"next"}; !slices.Equal(got, want) {
		t.Fatalf("physical lines = %q, want %q", got, want)
	}
	if sb.discardingLogicalLine {
		t.Fatal("discard state should end at a hard line boundary")
	}
}

func TestSetLogicalScrollbackLimitDropsExistingPartialHead(t *testing.T) {
	sb := NewScrollback(3)
	sb.push(lineFromString("aaaa"), true, 4)
	sb.push(lineFromString("bbbb"), true, 4)
	sb.push(lineFromString("cccc"), false, 4)
	sb.push(lineFromString("next"), false, 4)

	if !sb.headPartial {
		t.Fatal("physical eviction should record a partial oldest line")
	}
	sb.SetMaxLogicalLines(10)
	if got, want := scrollbackStrings(sb), []string{"next"}; !slices.Equal(got, want) {
		t.Fatalf("physical lines = %q, want %q", got, want)
	}
}

func TestPhysicalScrollbackLimitStillEvictsRows(t *testing.T) {
	sb := NewScrollback(2)
	sb.push(lineFromString("aaaa"), true, 4)
	sb.push(lineFromString("bbbb"), true, 4)
	sb.push(lineFromString("cccc"), false, 4)

	if got, want := scrollbackStrings(sb), []string{"bbbb", "cccc"}; !slices.Equal(got, want) {
		t.Fatalf("physical lines = %q, want %q", got, want)
	}
}

func TestLogicalScrollbackSurvivesNarrowAndWideResize(t *testing.T) {
	e := NewEmulator(8, 2)
	e.SetScrollbackSize(4)
	e.SetLogicalScrollbackSize(2)
	_, _ = e.WriteString("abcdefghij\r\nklmnopqrst\r\nuvwxyzABCD")

	e.Resize(4, 2)
	if got, want := nonEmptyLogicalLines(e.PhysicalLines()), []string{"klmnopqrst", "uvwxyzABCD"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines after narrowing = %q, want %q", got, want)
	}
	if got := e.ScrollbackLen(); got > 4 {
		t.Fatalf("physical scrollback length = %d, want at most 4", got)
	}

	e.Resize(12, 2)
	if got, want := nonEmptyLogicalLines(e.PhysicalLines()), []string{"klmnopqrst", "uvwxyzABCD"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines after widening = %q, want %q", got, want)
	}
}

func TestLogicalScrollbackWideCharactersAndHardBreaks(t *testing.T) {
	e := NewEmulator(4, 2)
	e.SetLogicalScrollbackSize(2)
	_, _ = e.WriteString("你你你\r\nabcdef\r\n好e\u0301好")

	logical := nonEmptyLogicalLines(e.PhysicalLines())
	if got, want := logical[len(logical)-2:], []string{"abcdef", "好e\u0301好"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines = %q, want %q", got, want)
	}
}

func TestLogicalScrollbackED2KeepsBlankContinuation(t *testing.T) {
	e := NewEmulator(4, 3)
	e.SetLogicalScrollbackSize(10)
	_, _ = e.WriteString("abc     ")
	_, _ = e.WriteString("\x1b[2J")

	if got, want := logicalLineStrings(e.PhysicalLines())[0], "abc     "; got != want {
		t.Fatalf("logical line after ED 2 = %q, want %q", got, want)
	}
}

func TestLogicalScrollbackAlternateScreenIsIndependent(t *testing.T) {
	e := NewEmulator(4, 2)
	e.SetLogicalScrollbackSize(1)
	_, _ = e.WriteString("main-one\r\nmain-two")
	before := scrollbackStrings(e.Scrollback())

	_, _ = e.WriteString("\x1b[?1049h")
	_, _ = e.WriteString("alternate-screen-content")
	if got := scrollbackStrings(e.Scrollback()); !slices.Equal(got, before) {
		t.Fatalf("main scrollback changed on alternate screen: got %q, want %q", got, before)
	}
}

func lineFromString(text string) uv.Line {
	line := make(uv.Line, 0, len([]rune(text)))
	for _, r := range text {
		line = append(line, uv.Cell{Content: string(r), Width: 1})
	}
	return line
}

func scrollbackStrings(sb *Scrollback) []string {
	lines := make([]string, sb.Len())
	for i := range lines {
		lines[i] = sb.Line(i).String()
	}
	return lines
}

func nonEmptyLogicalLines(lines []PhysicalLine) []string {
	logical := logicalLineStrings(lines)
	return slices.DeleteFunc(logical, func(line string) bool { return line == "" })
}

func TestResizeReflowsSoftWrappedLines(t *testing.T) {
	e := NewEmulator(12, 4)
	const text = "one two three four"
	_, _ = e.WriteString(text)

	e.Resize(6, 4)
	if got := strings.ReplaceAll(e.String(), "\n", ""); got != text {
		t.Fatalf("content after shrinking = %q, want %q", got, text)
	}

	e.Resize(24, 4)
	if got := strings.TrimRight(e.String(), "\n"); got != text {
		t.Fatalf("content after expanding = %q, want %q", got, text)
	}
}

func TestResizePreservesHardLineBreaks(t *testing.T) {
	e := NewEmulator(8, 4)
	_, _ = e.WriteString("abcdefghij\r\nsecond")

	e.Resize(20, 4)
	if got, want := strings.TrimRight(e.String(), "\n"), "abcdefghij\nsecond"; got != want {
		t.Fatalf("resized screen = %q, want %q", got, want)
	}
}

func TestResizeReflowsAcrossScrollback(t *testing.T) {
	e := NewEmulator(5, 2)
	const text = "abcdefghijklmnop"
	_, _ = e.WriteString(text)
	if e.ScrollbackLen() == 0 {
		t.Fatal("expected wrapped content in scrollback before resize")
	}

	e.Resize(20, 2)
	if got := strings.TrimRight(e.String(), "\n"); got != text {
		t.Fatalf("expanded screen = %q, want %q", got, text)
	}
	if got := e.ScrollbackLen(); got != 0 {
		t.Fatalf("scrollback length after expanding = %d, want 0", got)
	}
}

func TestResizePreservesCursorInReflowedLine(t *testing.T) {
	e := NewEmulator(10, 3)
	_, _ = e.WriteString("abcdefgh")
	e.Resize(4, 3)
	_, _ = e.WriteString("Z")
	e.Resize(20, 3)

	if got, want := strings.TrimRight(e.String(), "\n"), "abcdefghZ"; got != want {
		t.Fatalf("screen after writing at reflowed cursor = %q, want %q", got, want)
	}
}

func TestResizePreservesPendingWrapCursor(t *testing.T) {
	e := NewEmulator(5, 2)
	_, _ = e.WriteString("abcde")

	e.Resize(10, 2)
	_, _ = e.WriteString("f")
	if got, want := strings.TrimRight(e.String(), "\n"), "abcdef"; got != want {
		t.Fatalf("screen after expanding pending wrap = %q, want %q", got, want)
	}

	e.Resize(3, 2)
	_, _ = e.WriteString("g")
	e.Resize(10, 2)
	if got, want := strings.TrimRight(e.String(), "\n"), "abcdefg"; got != want {
		t.Fatalf("screen after shrinking pending wrap = %q, want %q", got, want)
	}
}

func TestResizeMovesLinesBetweenScreenAndScrollback(t *testing.T) {
	e := NewEmulator(8, 4)
	_, _ = e.WriteString("one\r\ntwo\r\nthree\r\nfour")

	e.Resize(8, 2)
	if got, want := e.ScrollbackLen(), 2; got != want {
		t.Fatalf("scrollback length after shrinking = %d, want %d", got, want)
	}
	if got, want := e.String(), "three\nfour"; got != want {
		t.Fatalf("screen after shrinking = %q, want %q", got, want)
	}

	e.Resize(8, 4)
	if got := e.ScrollbackLen(); got != 0 {
		t.Fatalf("scrollback length after expanding = %d, want 0", got)
	}
	if got, want := e.String(), "one\ntwo\nthree\nfour"; got != want {
		t.Fatalf("screen after expanding = %q, want %q", got, want)
	}
}

func TestResizePreservesSpacesAtSoftWrap(t *testing.T) {
	e := NewEmulator(5, 3)
	const text = "abc  def"
	_, _ = e.WriteString(text)

	e.Resize(12, 3)
	if got := strings.TrimRight(e.String(), "\n"); got != text {
		t.Fatalf("expanded screen = %q, want %q", got, text)
	}
}

func TestResizeReflowsWideCharacters(t *testing.T) {
	e := NewEmulator(6, 3)
	const text = "你好世界"
	_, _ = e.WriteString(text)

	if got, want := e.String(), "你好世\n界\n"; got != want {
		t.Fatalf("initial wrapped screen = %q, want %q", got, want)
	}

	e.Resize(4, 5)
	e.Resize(12, 3)
	if got := strings.TrimRight(e.String(), "\n"); got != text {
		t.Fatalf("expanded screen = %q, want %q", got, text)
	}
}

func TestResizeDoesNotRestoreAltScreenScrollback(t *testing.T) {
	e := NewEmulator(8, 2)
	_, _ = e.WriteString("\x1b[?1049h")
	_, _ = e.WriteString("one\r\ntwo\r\nthree")

	e.Resize(8, 3)
	if got, want := e.String(), "two\nthree\n"; got != want {
		t.Fatalf("resized alternate screen = %q, want %q", got, want)
	}
}
