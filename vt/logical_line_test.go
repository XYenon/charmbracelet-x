package vt

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func logicalStrings(lines []LogicalLine) []string {
	strings := make([]string, len(lines))
	for i, line := range lines {
		strings[i] = line.String()
	}
	return strings
}

func TestLogicalLinesJoinsSoftWrapsAndPreservesHardBreaks(t *testing.T) {
	e := NewEmulator(4, 4)
	_, _ = e.WriteString("abc def\r\n你e\u0301好")

	if got, want := logicalStrings(e.LogicalLines())[:2], []string{"abc def", "你e\u0301好"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines = %q, want %q", got, want)
	}
}

func TestLogicalLinesSnapshotIsImmutable(t *testing.T) {
	e := NewEmulator(4, 2)
	_, _ = e.WriteString("abcdef")
	lines := e.LogicalLines()
	_, _ = e.WriteString("g")

	if got, want := lines[0].String(), "abcdef"; got != want {
		t.Fatalf("snapshot changed after write: got %q, want %q", got, want)
	}
}

func TestLogicalLinesPreservePendingWrapSpace(t *testing.T) {
	e := NewEmulator(4, 2)
	_, _ = e.WriteString("abc ")

	if got, want := e.LogicalLines()[0].String(), "abc "; got != want {
		t.Fatalf("logical line = %q, want %q", got, want)
	}
}

func TestLogicalLinesPreservePendingWrapSpaceThroughED2(t *testing.T) {
	e := NewEmulator(4, 2)
	_, _ = e.WriteString("abc ")
	_, _ = e.WriteString("\x1b[2J")

	if got, want := e.LogicalLines()[0].String(), "abc "; got != want {
		t.Fatalf("logical line after ED 2 = %q, want %q", got, want)
	}
}

func TestLogicalLinesCapacityDoesNotReturnPartialSoftWrap(t *testing.T) {
	e := NewEmulator(4, 2)
	e.SetScrollbackSize(3)
	e.SetLogicalScrollbackSize(10)
	_, _ = e.WriteString("abcdefgh\r\nijklmnop\r\nqrstuvwx")

	if got, want := nonEmptyLogicalStrings(e.LogicalLines()), []string{"ijklmnop", "qrstuvwx"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines = %q, want %q", got, want)
	}
}

func TestLogicalLinesOmitOversizedLogicalLine(t *testing.T) {
	e := NewEmulator(4, 2)
	e.SetScrollbackSize(2)
	e.SetLogicalScrollbackSize(10)
	_, _ = e.WriteString("abcdefghijklmnop\r\nnext")

	if got, want := nonEmptyLogicalStrings(e.LogicalLines()), []string{"next"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines = %q, want %q", got, want)
	}
}

func TestLogicalLinesNearCapacitySurviveResize(t *testing.T) {
	e := NewEmulator(8, 2)
	e.SetScrollbackSize(4)
	e.SetLogicalScrollbackSize(2)
	_, _ = e.WriteString("abcdefghij\r\nklmnopqrst\r\nuvwxyzABCD")

	e.Resize(4, 2)
	if got, want := nonEmptyLogicalStrings(e.LogicalLines()), []string{"klmnopqrst", "uvwxyzABCD"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines after narrowing = %q, want %q", got, want)
	}
	e.Resize(12, 2)
	if got, want := nonEmptyLogicalStrings(e.LogicalLines()), []string{"klmnopqrst", "uvwxyzABCD"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines after widening = %q, want %q", got, want)
	}
}

func TestLogicalLinesAlternateScreenBoundary(t *testing.T) {
	e := NewEmulator(5, 2)
	_, _ = e.WriteString("main1\r\nmain2\r\nmain3")
	_, _ = e.WriteString("\x1b[?1049h")
	_, _ = e.WriteString("abcdef")

	if got, want := nonEmptyLogicalStrings(e.LogicalLines()), []string{"main1", "abcdef"}; !slices.Equal(got, want) {
		t.Fatalf("logical lines = %q, want %q", got, want)
	}
}

func TestSafeEmulatorLogicalLines(t *testing.T) {
	e := NewSafeEmulator(4, 2)
	_, _ = e.Write([]byte("abcdef"))
	lines := e.LogicalLines()
	_, _ = e.Write([]byte("g"))

	if got, want := lines[0].String(), "abcdef"; got != want {
		t.Fatalf("safe snapshot changed after write: got %q, want %q", got, want)
	}
}

func nonEmptyLogicalStrings(lines []LogicalLine) []string {
	return slices.DeleteFunc(logicalStrings(lines), func(line string) bool { return line == "" })
}

func ExampleEmulator_LogicalLines() {
	e := NewEmulator(20, 2)
	e.SetLogicalScrollbackSize(2)
	_, _ = e.WriteString("one\r\ntwo\r\nthree")

	lines := e.LogicalLines()
	if len(lines) > 2 {
		lines = lines[len(lines)-2:]
	}
	text := make([]string, len(lines))
	for i, line := range lines {
		text[i] = line.String()
	}
	fmt.Println(strings.Join(text, "\n"))
	// Output:
	// two
	// three
}
