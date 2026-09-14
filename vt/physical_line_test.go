package vt

import (
	"fmt"
	"strings"
	"testing"
)

func logicalLineStrings(lines []PhysicalLine) []string {
	var logical []string
	var current strings.Builder
	for _, line := range lines {
		current.WriteString(line.String())
		if !line.Wrapped() {
			logical = append(logical, current.String())
			current.Reset()
		}
	}
	return logical
}

func TestPhysicalLinesScreen(t *testing.T) {
	e := NewEmulator(5, 3)
	_, _ = e.WriteString("abcdefgh")

	lines := e.PhysicalLines()
	if got, want := len(lines), 3; got != want {
		t.Fatalf("physical line count = %d, want %d", got, want)
	}
	if got, want := lines[0].String(), "abcde"; got != want {
		t.Fatalf("first physical line = %q, want %q", got, want)
	}
	if !lines[0].Wrapped() {
		t.Fatal("first physical line should continue onto the second")
	}
	if got, want := lines[1].String(), "fgh"; got != want {
		t.Fatalf("second physical line = %q, want %q", got, want)
	}
	if lines[1].Wrapped() {
		t.Fatal("second physical line should end the logical line")
	}

	cells := lines[0].Cells()
	cells[0].Content = "X"
	if got, want := e.PhysicalLines()[0].String(), "abcde"; got != want {
		t.Fatalf("mutating snapshot changed emulator: got %q, want %q", got, want)
	}
}

func TestPhysicalLinesAcrossScrollback(t *testing.T) {
	e := NewEmulator(5, 2)
	const text = "abcdefghijklmnop"
	_, _ = e.WriteString(text)

	lines := e.PhysicalLines()
	if got, want := len(lines), 4; got != want {
		t.Fatalf("physical line count = %d, want %d", got, want)
	}
	for i := 0; i < len(lines)-1; i++ {
		if !lines[i].Wrapped() {
			t.Fatalf("physical line %d should continue", i)
		}
	}
	if lines[len(lines)-1].Wrapped() {
		t.Fatal("last physical line must not continue beyond the snapshot")
	}
	if got, want := logicalLineStrings(lines)[0], text; got != want {
		t.Fatalf("logical line = %q, want %q", got, want)
	}
}

func TestPhysicalLinesAfterResize(t *testing.T) {
	e := NewEmulator(10, 3)
	const text = "abcdefgh"
	_, _ = e.WriteString(text)

	e.Resize(4, 3)
	lines := e.PhysicalLines()
	if !lines[0].Wrapped() || lines[1].Wrapped() {
		t.Fatalf("unexpected wrapped flags after shrinking: %v, %v", lines[0].Wrapped(), lines[1].Wrapped())
	}
	if got, want := logicalLineStrings(lines)[0], text; got != want {
		t.Fatalf("logical line after shrinking = %q, want %q", got, want)
	}

	e.Resize(12, 3)
	lines = e.PhysicalLines()
	if lines[0].Wrapped() {
		t.Fatal("expanded logical line should fit one physical line")
	}
	if got, want := lines[0].String(), text; got != want {
		t.Fatalf("physical line after expanding = %q, want %q", got, want)
	}
}

func TestPhysicalLinesPreserveHardBreaks(t *testing.T) {
	e := NewEmulator(4, 4)
	_, _ = e.WriteString("abcdef\r\nxy")

	lines := e.PhysicalLines()
	if !lines[0].Wrapped() || lines[1].Wrapped() || lines[2].Wrapped() {
		t.Fatalf("unexpected wrapped flags: %v, %v, %v", lines[0].Wrapped(), lines[1].Wrapped(), lines[2].Wrapped())
	}
	logical := logicalLineStrings(lines)
	if got, want := logical[0], "abcdef"; got != want {
		t.Fatalf("first logical line = %q, want %q", got, want)
	}
	if got, want := logical[1], "xy"; got != want {
		t.Fatalf("second logical line = %q, want %q", got, want)
	}
}

func TestPhysicalLinesAlternateScreen(t *testing.T) {
	e := NewEmulator(5, 2)
	_, _ = e.WriteString("main1\r\nmain2\r\nmain3")
	_, _ = e.WriteString("\x1b[?1049h")
	_, _ = e.WriteString("abcdef")

	lines := e.PhysicalLines()
	if got, want := lines[0].String(), "main1"; got != want {
		t.Fatalf("main scrollback line = %q, want %q", got, want)
	}
	if lines[0].Wrapped() {
		t.Fatal("main scrollback must not continue into the alternate screen")
	}
	if got, want := lines[1].String(), "abcde"; got != want {
		t.Fatalf("first alternate-screen line = %q, want %q", got, want)
	}
	if !lines[1].Wrapped() {
		t.Fatal("first alternate-screen line should continue")
	}
	if got, want := lines[2].String(), "f"; got != want {
		t.Fatalf("second alternate-screen line = %q, want %q", got, want)
	}
}

func TestPhysicalLinesWideAndCombiningCharacters(t *testing.T) {
	e := NewEmulator(4, 3)
	const text = "你e\u0301好"
	_, _ = e.WriteString(text)

	lines := e.PhysicalLines()
	if !lines[0].Wrapped() || lines[1].Wrapped() {
		t.Fatalf("unexpected wrapped flags: %v, %v", lines[0].Wrapped(), lines[1].Wrapped())
	}
	if got, want := logicalLineStrings(lines)[0], text; got != want {
		t.Fatalf("logical line = %q, want %q", got, want)
	}
}

func TestPhysicalLinesPendingWrap(t *testing.T) {
	e := NewEmulator(5, 2)
	_, _ = e.WriteString("abcde")
	if e.PhysicalLines()[0].Wrapped() {
		t.Fatal("pending wrap without a following character is not a continuation")
	}

	_, _ = e.WriteString("f")
	if !e.PhysicalLines()[0].Wrapped() {
		t.Fatal("writing after pending wrap should mark the first line as continued")
	}
}

func TestPhysicalLinesPreserveSpacesAtWrap(t *testing.T) {
	testCases := []struct {
		name  string
		width int
		text  string
	}{
		{name: "single space", width: 4, text: "abc def"},
		{name: "consecutive spaces", width: 4, text: "ab  cd"},
		{name: "space in scrollback", width: 4, text: "abc defghijkl"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEmulator(tc.width, 2)
			_, _ = e.WriteString(tc.text)
			if got, want := logicalLineStrings(e.PhysicalLines())[0], tc.text; got != want {
				t.Fatalf("logical line = %q, want %q", got, want)
			}
		})
	}
}

func TestPhysicalLinesExcludeUnusedSpaceBeforeWideCharacter(t *testing.T) {
	e := NewEmulator(4, 2)
	const text = "abc你"
	_, _ = e.WriteString(text)

	lines := e.PhysicalLines()
	if got, want := lines[0].String(), "abc"; got != want {
		t.Fatalf("first physical line = %q, want %q", got, want)
	}
	if got, want := logicalLineStrings(lines)[0], text; got != want {
		t.Fatalf("logical line = %q, want %q", got, want)
	}

	e.Resize(3, 3)
	e.Resize(8, 2)
	if got, want := logicalLineStrings(e.PhysicalLines())[0], text; got != want {
		t.Fatalf("logical line after resize = %q, want %q", got, want)
	}
}

func TestPhysicalLinesPreserveSpacesThroughResize(t *testing.T) {
	e := NewEmulator(4, 3)
	const text = "abc def"
	_, _ = e.WriteString(text)

	e.Resize(3, 3)
	if got, want := logicalLineStrings(e.PhysicalLines())[0], text; got != want {
		t.Fatalf("logical line after shrinking = %q, want %q", got, want)
	}

	e.Resize(8, 3)
	if got, want := logicalLineStrings(e.PhysicalLines())[0], text; got != want {
		t.Fatalf("logical line after expanding = %q, want %q", got, want)
	}
}

func TestPhysicalLinesPreservePendingWrapSpace(t *testing.T) {
	e := NewEmulator(4, 2)
	_, _ = e.WriteString("abc ")

	line := e.PhysicalLines()[0]
	if line.Wrapped() {
		t.Fatal("pending wrap should not report a continuation")
	}
	if got, want := line.String(), "abc "; got != want {
		t.Fatalf("pending-wrap line = %q, want %q", got, want)
	}
}

func TestSafeEmulatorPhysicalLines(t *testing.T) {
	e := NewSafeEmulator(5, 2)
	_, _ = e.Write([]byte("abcdef"))
	lines := e.PhysicalLines()
	cells := lines[0].Cells()
	cells[0].Content = "X"
	if got, want := e.PhysicalLines()[0].String(), "abcde"; got != want {
		t.Fatalf("mutating safe snapshot changed emulator: got %q, want %q", got, want)
	}
}

func TestSafeEmulatorSetLogicalScrollbackSize(t *testing.T) {
	e := NewSafeEmulator(4, 2)
	e.SetLogicalScrollbackSize(2)
	if got, want := e.Emulator.Scrollback().maxLogicalLines, 2; got != want {
		t.Fatalf("logical scrollback size = %d, want %d", got, want)
	}
}

func ExampleEmulator_PhysicalLines() {
	e := NewEmulator(5, 3)
	e.SetLogicalScrollbackSize(100)
	_, _ = e.WriteString("abcdefgh\r\nxy")

	var current strings.Builder
	for _, line := range e.PhysicalLines() {
		current.WriteString(line.String())
		if !line.Wrapped() {
			fmt.Println(current.String())
			current.Reset()
		}
	}
	// Output:
	// abcdefgh
	// xy
}
