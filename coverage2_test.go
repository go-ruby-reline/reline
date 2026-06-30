package reline

import "testing"

func TestViYankDoubled(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// yy -> yank whole line to clipboard
	feed(le, keyEv{"y", "vi_yank"})
	feed(le, keyEv{"y", "vi_yank"})
	// paste it after
	feed(le, keyEv{"p", "vi_paste_next"})
	if le.WholeBuffer() == "hello" {
		t.Fatalf("yy then p did nothing: %q", le.WholeBuffer())
	}
}

func TestViChangeMetaDoubled(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// cc -> change whole line
	feed(le, keyEv{"c", "vi_change_meta"})
	feed(le, keyEv{"c", "vi_change_meta"})
	// MRI: cc clears the line via the doubled-operator path but does NOT switch
	// to insert mode (that only happens through the motion+operator route).
	if le.WholeBuffer() != "" {
		t.Fatalf("cc=%q", le.WholeBuffer())
	}
}

func TestViChangeMetaWithArgNoReset(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("aa bb cc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// 2cc - operator with arg, then operator again (the arg!=nil doubled branch)
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"c", "vi_change_meta"})
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"c", "vi_change_meta"})
	// arg-present doubled operator does not blank the line
	if le.WholeBuffer() != "aa bb cc" {
		t.Fatalf("2c2c=%q", le.WholeBuffer())
	}
}

func TestEdNewlineViCommandMultiline(t *testing.T) {
	le := newViEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("b")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	le.lineIndex = 0
	// vi command newline on non-last line acts as cursor-down
	feed(le, keyEv{"\n", "ed_newline"})
	if le.LineIndex() != 1 {
		t.Fatalf("vi newline cursor down line=%d", le.LineIndex())
	}
	// on last line, vi command newline finishes
	feed(le, keyEv{"\n", "ed_newline"})
	if !le.Finished() {
		t.Fatal("vi newline last line should finish")
	}
}

func TestPromptListWithArg(t *testing.T) {
	le := newViEditor()
	le.MultilineOn()
	feed(le, insertStr("x")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"5", "ed_argument_digit"})
	pp := func(lines []string) []string {
		out := make([]string, len(lines))
		for i := range lines {
			out[i] = "P> "
		}
		return out
	}
	rs := le.Render(80, pp)
	// with vi_arg active, prompt is the arg prompt
	if rs.Lines[0][0] != "(arg: 5) " {
		t.Fatalf("arg prompt=%q", rs.Lines[0][0])
	}
}

func TestPromptListEmptyProc(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("b")...)
	// proc returns fewer prompts than lines -> last is repeated
	pp := func(lines []string) []string { return []string{"only> "} }
	rs := le.Render(80, pp)
	if rs.Lines[1][0] != "only> " {
		t.Fatalf("repeated prompt=%q", rs.Lines[1][0])
	}
}

func TestPromptListEmptyResult(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	pp := func(lines []string) []string { return nil }
	rs := le.Render(80, pp)
	if rs.Lines[0][0] != "> " {
		t.Fatalf("empty proc result prompt=%q", rs.Lines[0][0])
	}
}

func TestEdDigitWithArgInVi(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abcdefghij")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// In vi command, '1' '2' are bound to ed_digit which routes to argument
	feed(le, keyEv{"1", "ed_digit"})
	feed(le, keyEv{"2", "ed_digit"})
	feed(le, keyEv{"l", "ed_next_char"})
	// 12l moves right (clamped to end-1 in vi)
	if le.BytePointer() == 0 {
		t.Fatal("12l should move")
	}
}

func TestNthLineOutOfRange(t *testing.T) {
	if nthLine([]string{"a"}, 5) != "" {
		t.Fatal("out of range nthLine")
	}
	if nthLine([]string{"a", "b"}, 1) != "b" {
		t.Fatal("in range nthLine")
	}
}

func TestFirstRuneEmpty(t *testing.T) {
	if firstRune("") != 0 {
		t.Fatal("empty firstRune")
	}
	if firstRune("abc") != 'a' {
		t.Fatal("firstRune")
	}
}

func TestFirstDigitNone(t *testing.T) {
	if firstDigit("abc") != 0 {
		t.Fatal("no digit")
	}
	if firstDigit("x5y") != 5 {
		t.Fatal("digit")
	}
}

func TestGetMbcharWidthControl(t *testing.T) {
	if getMbcharWidth("\x01") != 2 {
		t.Fatal("control width")
	}
	if getMbcharWidth("a") != 1 {
		t.Fatal("ascii width")
	}
	if getMbcharWidth("漢") != 2 {
		t.Fatal("wide width")
	}
}

func TestBytesliceEdges(t *testing.T) {
	if byteslice("hello", 10, 3) != "" {
		t.Fatal("off past end")
	}
	if byteslice("hello", 2, 100) != "llo" {
		t.Fatal("len past end")
	}
	if byteslice("hello", -1, 3) != "" {
		t.Fatal("negative off")
	}
	if byteslice("hello", 3, -5) != "" {
		t.Fatalf("negative size: %q", byteslice("hello", 3, -5))
	}
}

func TestHistSliceClamp(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("a")
	le.hist.Append("b")
	s := le.histSlice(-5, 100)
	if len(s) != 2 {
		t.Fatalf("clamp=%v", s)
	}
}

func TestEmitGraphemeInvalidByte(t *testing.T) {
	// invalid UTF-8 start byte handled as single byte
	w := CalculateWidth("\xff", true)
	_ = w // just exercise emitGrapheme's invalid-byte branch
}

func TestMatchesPrintControlReject(t *testing.T) {
	if matchesPrint("\x01") {
		t.Fatal("control should not be printable")
	}
	if matchesPrint("") {
		t.Fatal("empty not printable")
	}
	if !matchesPrint("a") {
		t.Fatal("a printable")
	}
}

func TestIsLowerUpperGC(t *testing.T) {
	if !isLowerGC("a") || isLowerGC("A") || isLowerGC("1") {
		t.Fatal("isLowerGC")
	}
	if !isUpperGC("A") || isUpperGC("a") {
		t.Fatal("isUpperGC")
	}
}

func TestEdInsertUndefinedConversion(t *testing.T) {
	// ed_insert of a normal char path already covered; exercise continuous flush
	le, _, _ := newTestEditor()
	le.continuousInsertBuffer = "pre"
	feed(le, keyEv{"x", "ed_insert"})
	if le.WholeBuffer() != "prex" {
		t.Fatalf("flush continuous=%q", le.WholeBuffer())
	}
}

func TestViForwardWordSpaceStart(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("   abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"w", "vi_next_word"}) // from spaces to "abc"
	if le.BytePointer() != 3 {
		t.Fatalf("vi w from space=%d", le.BytePointer())
	}
}

func TestSearchNextHistoryEmptySubstr(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("aaa")
	le.hist.Append("bbb")
	le.hist.Append("ccc")
	// empty prefix search forward/back navigates like history
	feed(le, keyEv{"p", "ed_search_prev_history"})
	feed(le, keyEv{"p", "ed_search_prev_history"})
	feed(le, keyEv{"n", "ed_search_next_history"})
	if le.WholeBuffer() == "" {
		t.Fatal("empty-substr search navigated to empty")
	}
}

func TestViEndBigWordInclusiveArg(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("aa bb")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// dE -> delete to end of big word inclusive
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"E", "vi_end_big_word"})
	if le.WholeBuffer() != " bb" {
		t.Fatalf("dE=%q", le.WholeBuffer())
	}
}

func TestViEndWordInclusiveOperator(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// de -> delete to end of word inclusive
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"e", "vi_end_word"})
	if le.WholeBuffer() != " bar" {
		t.Fatalf("de=%q", le.WholeBuffer())
	}
}
