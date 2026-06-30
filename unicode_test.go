package reline

import "testing"

func TestCalculateWidthBasic(t *testing.T) {
	cases := []struct {
		s string
		w int
	}{
		{"hello", 5},
		{"漢字", 4},
		{"é", 1},
		{"", 0},
		{"a漢b", 4},
	}
	for _, c := range cases {
		if got := CalculateWidth(c.s, false); got != c.w {
			t.Errorf("width(%q)=%d want %d", c.s, got, c.w)
		}
	}
}

func TestCalculateWidthControlChars(t *testing.T) {
	// control char renders as 2-cell caret
	if w := CalculateWidth("\x01", false); w != 2 {
		t.Fatalf("ctrl width=%d", w)
	}
}

func TestCalculateWidthEscapeCodes(t *testing.T) {
	s := "\x1b[31mred\x1b[0m"
	if w := CalculateWidth(s, true); w != 3 {
		t.Fatalf("escape width=%d", w)
	}
	// non-printing markers
	s2 := "\x01ignored\x02visible"
	if w := CalculateWidth(s2, true); w != 7 {
		t.Fatalf("nonprint width=%d", w)
	}
}

func TestCalculateWidthOSC(t *testing.T) {
	s := "\x1b]0;title\x07text"
	if w := CalculateWidth(s, true); w != 4 {
		t.Fatalf("osc width=%d", w)
	}
	// OSC terminated with ST (ESC \), single parameter segment.
	s2 := "\x1b]2;title\x1b\\link"
	if w := CalculateWidth(s2, true); w != 4 {
		t.Fatalf("osc-st width=%d", w)
	}
}

func TestSplitByWidth(t *testing.T) {
	lines, n := SplitByWidth("aaaaaa", 3)
	if n < 2 {
		t.Fatalf("n=%d", n)
	}
	if lines[0] != "aaa" {
		t.Fatalf("first=%q", lines[0])
	}
}

func TestSplitLineByWidthColorCarry(t *testing.T) {
	// CSI color should carry to wrapped lines
	lines := SplitLineByWidth("\x1b[31maaaa", 2, 0)
	if len(lines) < 2 {
		t.Fatalf("lines=%v", lines)
	}
}

func TestSplitLineExactWidth(t *testing.T) {
	lines := SplitLineByWidth("ab", 2, 0)
	// width == max_width appends an empty trailing line
	if lines[len(lines)-1] != "" {
		t.Fatalf("trailing=%q", lines[len(lines)-1])
	}
}

func TestEscapeForPrint(t *testing.T) {
	if got := EscapeForPrint("a\tb"); got != "a  b" {
		t.Fatalf("tab=%q", got)
	}
	if got := EscapeForPrint("a\nb"); got != "a\nb" {
		t.Fatalf("newline=%q", got)
	}
	if got := EscapeForPrint("\x01"); got != "^A" {
		t.Fatalf("ctrl=%q", got)
	}
}

func TestCommonPrefix(t *testing.T) {
	if got := CommonPrefix([]string{"foobar", "foobaz"}, false); got != "fooba" {
		t.Fatalf("prefix=%q", got)
	}
	if got := CommonPrefix(nil, false); got != "" {
		t.Fatalf("empty=%q", got)
	}
	if got := CommonPrefix([]string{"FOO", "foobar"}, true); got != "FOO" {
		t.Fatalf("ignorecase=%q", got)
	}
	if got := CommonPrefix([]string{"abc", "xyz"}, false); got != "" {
		t.Fatalf("no common=%q", got)
	}
}

func TestGraphemeClustersCombining(t *testing.T) {
	// "e" + combining acute accent forms one grapheme
	s := "é"
	gcs := graphemeClusters(s)
	if len(gcs) != 1 {
		t.Fatalf("combining gcs=%d: %v", len(gcs), gcs)
	}
	if w := CalculateWidth(s, false); w != 1 {
		t.Fatalf("combining width=%d", w)
	}
}

func TestGraphemeClustersZWJ(t *testing.T) {
	// family emoji with ZWJ — at least joins without exploding cell count wildly
	s := "👩‍💻"
	gcs := graphemeClusters(s)
	if len(gcs) != 1 {
		t.Fatalf("zwj gcs=%d", len(gcs))
	}
}

func TestGraphemeClustersRegionalIndicator(t *testing.T) {
	flag := "\U0001F1EB\U0001F1F7" // FR flag
	gcs := graphemeClusters(flag)
	if len(gcs) != 1 {
		t.Fatalf("flag gcs=%d", len(gcs))
	}
}

func TestGraphemeClustersEmpty(t *testing.T) {
	if graphemeClusters("") != nil {
		t.Fatal("empty should be nil")
	}
}

func TestWordAndSpaceCharacter(t *testing.T) {
	if !wordCharacter("a") || !wordCharacter("_") || !wordCharacter("9") {
		t.Fatal("word chars")
	}
	if wordCharacter(" ") || wordCharacter("") {
		t.Fatal("non-word")
	}
	if !spaceCharacter(" ") || !spaceCharacter("\t") {
		t.Fatal("space chars")
	}
	if spaceCharacter("a") || spaceCharacter("") {
		t.Fatal("non-space")
	}
}

func TestCapitalize(t *testing.T) {
	if capitalize("fooBAR") != "Foobar" {
		t.Fatalf("cap=%q", capitalize("fooBAR"))
	}
	if capitalize("") != "" {
		t.Fatal("empty cap")
	}
}

func TestEastAsianWidthAmbiguous(t *testing.T) {
	old := AmbiguousWidth
	defer func() { AmbiguousWidth = old }()
	AmbiguousWidth = 2
	// U+00A1 (¡) is ambiguous width
	if eastAsianWidth(0x00A1) != 2 {
		t.Fatalf("ambiguous=%d", eastAsianWidth(0x00A1))
	}
}

func TestGetPrevNextMbcharSize(t *testing.T) {
	line := "a漢b"
	if getNextMbcharSize(line, 0) != 1 {
		t.Fatal("next size a")
	}
	if getNextMbcharSize(line, 1) != 3 {
		t.Fatal("next size 漢")
	}
	if getNextMbcharSize(line, len(line)) != 0 {
		t.Fatal("next size at end")
	}
	if getPrevMbcharSize(line, 0) != 0 {
		t.Fatal("prev size at start")
	}
	if getPrevMbcharSize(line, 4) != 3 {
		t.Fatalf("prev size 漢=%d", getPrevMbcharSize(line, 4))
	}
}
