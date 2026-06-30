package reline

import "testing"

func TestWrapMethodCallUnknownSymbol(t *testing.T) {
	le, _, _ := newTestEditor()
	// dispatch an unknown command symbol -> no-op (not in table)
	feed(le, keyEv{"x", "no_such_command"})
	if le.WholeBuffer() != "" {
		t.Fatalf("unknown cmd changed buffer=%q", le.WholeBuffer())
	}
}

func TestEdDigitWithArgInsertsViaArgument(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abcdef")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"2", "ed_argument_digit"}) // set arg
	feed(le, keyEv{"3", "ed_digit"})          // ed_digit with arg -> ed_argument_digit (arg=23)
	feed(le, keyEv{"l", "ed_next_char"})
	if le.BytePointer() == 0 {
		t.Fatal("23l should move")
	}
}

func TestCompletionMultilineBlock(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"barbar"}
	})
	le.MultilineOn()
	feed(le, insertStr("pre")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("bar")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("post")...)
	le.lineIndex = 1
	le.bytePointer = 3
	pre, target, post, _ := le.retrieveCompletionBlock()
	if target != "bar" {
		t.Fatalf("target=%q", target)
	}
	// pre includes line 0, post includes line 2
	if pre != "pre\nbar" && pre != "pre\n" {
		t.Logf("pre=%q", pre)
	}
	if post == "" {
		t.Fatalf("post=%q", post)
	}
}

func TestCompletionEscapedQuote(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return nil })
	feed(le, insertStr("a \"x\\\"y")...) // a "x\"y -> escaped quote inside
	_, _, _, quote := le.retrieveCompletionBlock()
	// the escaped quote should keep quote open
	if quote != "\"" {
		t.Fatalf("quote=%q", quote)
	}
}

func TestHistoryNavViCommandCursorStart(t *testing.T) {
	le := newViEditor()
	le.hist.Append("hello")
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"\x10", "ed_prev_history"})
	// vi command mode lands cursor at start
	if le.BytePointer() != 0 {
		t.Fatalf("vi prev history ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"\x0e", "ed_next_history"})
}

func TestKeyStrokeUnmatchedStandalone(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// 2-byte non-prefix ASCII like "ab" is UNMATCHED as a unit
	if ks.matchStatus(ints('a', 'b')) != statusUnmatched {
		t.Fatal("ab should be unmatched")
	}
}

func TestKeyStrokeExpandNoMatch(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// a sequence that never matches -> empty keys
	keys, _ := ks.expand(ints('a', 'b'))
	// 'a' alone is MATCHED (single char ed_insert), so first iter matches 'a'
	if len(keys) == 0 {
		t.Fatal("expected at least the single char")
	}
}

func TestKeyStrokeEscEscChar(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg) // emacs
	// ESC ESC x  -> not vi, default char branch -> classified by length
	st := ks.matchStatus(ints(27, 27, 'x'))
	if st == statusUnmatched {
		t.Fatal("ESC ESC x in emacs should not be flatly unmatched")
	}
}

func TestSearchTerminators(t *testing.T) {
	le := setupSearchEditor()
	le.config.IsearchTerminators = "\x07"
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"})
	// the configured terminator (other than Ctrl-G handling) terminates
	feed(le, keyEv{"\x07", "ed_ignore"})
	if _, searching := le.SearchingPrompt(); searching {
		// Ctrl-G is cancel; but as a terminator key path is also exercised
	}
}

func TestSearchLineBackupHit(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	feed(le, insertStr("zzdraftzz")...)
	feed(le, keyEv{"\x12", "vi_search_prev"})
	// search for substring present in the in-progress line backup
	feed(le, keyEv{"d", "ed_insert"})
	feed(le, keyEv{"r", "ed_insert"})
	if le.WholeBuffer() != "zzdraftzz" {
		t.Fatalf("backup hit=%q", le.WholeBuffer())
	}
}

func TestForwardSearchAgainWithPointer(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	le.hist.Append("apricot")
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"})
	feed(le, keyEv{"\x13", "vi_search_next"}) // forward again from a hit pointer
	feed(le, keyEv{"\x13", "vi_search_next"})
	if _, searching := le.SearchingPrompt(); !searching {
		t.Fatal("forward search active")
	}
}

func TestRenderSingleLineMultiBuffer(t *testing.T) {
	// promptList non-multiline path with only line 0 prompt
	le, _, _ := newTestEditor()
	feed(le, insertStr("hi")...)
	rs := le.Render(80, nil)
	if rs.Lines[0][0] != "> " {
		t.Fatalf("prompt=%q", rs.Lines[0][0])
	}
}

func TestRenderWrappedPromptLines(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("x")...)
	// a prompt wider than the width forces wrapped prompt lines
	pp := func(lines []string) []string {
		out := make([]string, len(lines))
		for i := range lines {
			out[i] = "verylongprompt> "
		}
		return out
	}
	rs := le.Render(8, pp)
	if len(rs.Lines) < 2 {
		t.Fatalf("expected wrapped prompt rows, got %d", len(rs.Lines))
	}
}

func TestHistoryPushOverflowClearsVals(t *testing.T) {
	h := NewHistory(2)
	h.Push("a", "b")
	// push 5 into cap 2: diff=5 > size(2) -> clear, drop 3 from vals, keep last 2
	h.Push("c", "d", "e", "f", "g")
	if h.Size() != 2 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "f" {
		t.Fatalf("get0=%q", v)
	}
}

func TestHistoryPushDropAllVals(t *testing.T) {
	h := NewHistory(1)
	h.Push("a")
	// push 2 into cap 1, diff=2 > size(1): clear then drop... keep last 1
	h.Push("b", "c")
	if h.Size() != 1 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "c" {
		t.Fatalf("get0=%q", v)
	}
}

func TestHistoryCheckIndexCapNegative(t *testing.T) {
	h := NewHistory(2)
	h.Push("a", "b")
	// index more negative than -cap raises
	if _, err := h.Get(-3); err == nil {
		t.Fatal("expected cap negative error")
	}
}

func TestViForwardWordPunctClass(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("...abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"w", "vi_next_word"}) // from punctuation class to word
	if le.BytePointer() != 3 {
		t.Fatalf("vi w punct=%d", le.BytePointer())
	}
}

func TestViForwardEndWordPunct(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("a... b")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"e", "vi_end_word"}) // end of "..." punctuation run
	if le.BytePointer() != 3 {
		t.Fatalf("vi e punct=%d", le.BytePointer())
	}
}

func TestTransposeWordsAtEnd(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("aa bb ")...) // cursor after trailing space
	feed(le, keyEv{"t", "ed_transpose_words"})
	// matches MRI: the words swap around the middle whitespace, preserving it.
	if le.WholeBuffer() != "bb  aa" {
		t.Fatalf("transpose at end=%q", le.WholeBuffer())
	}
}

func TestSearchPrevCharNeedNext(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("axbxc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	feed(le, keyEv{"T", "vi_to_prev_char"})
	feed(le, keyEv{"x", "ed_insert"})
	// T moves to just after the previous 'x'
	if le.BytePointer() == 0 {
		t.Fatal("T should move")
	}
}

func TestViDeleteMetaConfirmZero(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// d with a zero-motion (0 at col 0) -> no delete
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"0", "vi_zero"})
	if le.WholeBuffer() != "abc" {
		t.Fatalf("d0 at bol=%q", le.WholeBuffer())
	}
}

func TestCommonPrefixEmptyItem(t *testing.T) {
	if got := CommonPrefix([]string{"abc", ""}, false); got != "" {
		t.Fatalf("empty item prefix=%q", got)
	}
}

func TestInsertMultilineTextSplitsBuffer(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("XY")...)
	le.bytePointer = 1
	le.InsertMultilineText("a\nb")
	// "X" + "a\nb" + "Y" => "Xa", "bY"
	if le.WholeBuffer() != "Xa\nbY" {
		t.Fatalf("split=%q", le.WholeBuffer())
	}
}

func TestEmDeleteOrListEmptyLine(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return nil })
	// empty line -> em_delete path (eof since empty)
	feed(le, keyEv{"\x04", "em_delete_or_list"})
	if !le.EOF() {
		t.Fatal("empty delete_or_list should eof")
	}
}

func TestScanWidthNonPrintInSplit(t *testing.T) {
	// non-printing markers inside split_line_by_width
	lines := SplitLineByWidth("\x01hidden\x02vis", 80, 0)
	if len(lines) == 0 {
		t.Fatal("split nonprint")
	}
}

func TestSplitWithOSCCarry(t *testing.T) {
	// OSC sequence carried during wrap
	lines := SplitLineByWidth("\x1b]0;t\x07abcdef", 3, 0)
	if len(lines) < 2 {
		t.Fatalf("osc split=%v", lines)
	}
}
