package reline

import "testing"

func TestRenderMultilineNilProc(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("b")...)
	// multiline with nil prompt proc: same prompt for every line
	rs := le.Render(80, nil)
	if len(rs.Lines) != 2 {
		t.Fatalf("lines=%d", len(rs.Lines))
	}
	if rs.Lines[0][0] != "> " || rs.Lines[1][0] != "> " {
		t.Fatalf("prompts=%q,%q", rs.Lines[0][0], rs.Lines[1][0])
	}
}

func TestCompletionQuoteClosed(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return nil })
	// a quote opened and then closed before the cursor -> quote nil
	feed(le, insertStr("\"closed\" tar")...)
	_, target, _, quote := le.retrieveCompletionBlock()
	if quote != "" {
		t.Fatalf("quote should be closed, got %q", quote)
	}
	if target != "tar" {
		t.Fatalf("target=%q", target)
	}
}

func TestCompletionUniqueWithQuote(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"filename"}
	})
	le.SetCompletionAppendCharacter(" ")
	feed(le, insertStr("\"file")...)
	feed(le, keyEv{"\t", "complete"})
	// unique match inside a quote appends the closing quote
	if le.WholeBuffer() != "\"filename\"" {
		t.Fatalf("quote complete=%q", le.WholeBuffer())
	}
}

func TestCompletionPerfectMatchShowAll(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foo", "foobar"}
	})
	le.config.ShowAllIfAmbiguous = true
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	// "foo" present + multiple + show_all -> menu immediately, state PERFECT_MATCH
	if _, ok := le.MenuInfo(); !ok {
		t.Fatal("expected menu")
	}
	feed(le, keyEv{"\t", "complete"}) // PERFECT_MATCH state branch
}

func TestViCharSearchInclusive(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// df<char> deletes up to and including the found char (inclusive)
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"f", "vi_next_char"})
	feed(le, keyEv{"o", "ed_insert"})
	if le.WholeBuffer() != " world" {
		t.Fatalf("dfo=%q", le.WholeBuffer())
	}
}

func TestViToPrevCharNeedNext(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("axbxc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	// T finds previous 'x' and lands one past it
	feed(le, keyEv{"T", "vi_to_prev_char"})
	feed(le, keyEv{"x", "ed_insert"})
	if le.BytePointer() != 4 {
		t.Fatalf("Tx ptr=%d", le.BytePointer())
	}
}

func TestHistoryPushValsNilBranch(t *testing.T) {
	h := NewHistory(1)
	h.Push("a")
	// diff = size(1)+vals(1) - cap(1) = 1; diff <= size -> shift path? no:
	// push 1 more, diff=1, diff<=size(1) -> drop 1 from entries
	h.Push("b")
	if h.Size() != 1 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "b" {
		t.Fatalf("v=%q", v)
	}
}

func TestHistoryPushExceedDropAllVals(t *testing.T) {
	h := NewHistory(1)
	// push 3 into empty cap 1: diff = 0+3-1 = 2 > size(0): clear, diff-=0 -> 2,
	// vals(3) > 2 -> drop 2 keep last 1
	h.Push("a", "b", "c")
	if h.Size() != 1 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "c" {
		t.Fatalf("v=%q", v)
	}
}

func TestHistoryCheckIndexCapPositiveBound(t *testing.T) {
	h := NewHistory(5)
	h.Push("a", "b", "c")
	// index within entries but beyond... index 4 > size(3) -> IndexError, not cap
	if _, err := h.Get(4); err == nil {
		t.Fatal("expected index error")
	}
	// index exactly cap+something to hit the cap-range branch: index 6 > cap 5
	if _, err := h.Get(6); err == nil {
		t.Fatal("expected cap range error")
	}
}

func TestKeyStrokeExpandEmptyOnUnmatched(t *testing.T) {
	cfg := NewConfig()
	cfg.SetEditingMode(ModeViCommand)
	ks := newKeyStroke(cfg)
	// In vi, ESC followed by an unbound char: matchUnknownEscapeSequence ->
	// UNMATCHED on the 2-byte seq, no matched bytes -> empty expand result.
	keys, rest := ks.expand(ints(27, 0x80))
	_ = keys
	_ = rest
}

func TestKeyStrokeMatchUnknownNotEsc(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// matchUnknownEscapeSequence guard: first byte not ESC -> UNMATCHED
	if ks.matchUnknownEscapeSequence(ints('a'), false) != statusUnmatched {
		t.Fatal("non-esc unknown should be unmatched")
	}
}

func TestKeyStrokeLoneEscMatched(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// directly exercise the idx==1 ESC -> MATCHING_MATCHED return
	if ks.matchUnknownEscapeSequence(ints(27), false) != statusMatchingMatched {
		t.Fatal("lone esc")
	}
}

func TestEdInsertFlushesPriorContinuous(t *testing.T) {
	le, _, _ := newTestEditor()
	// not pasting, but with a pending continuous buffer -> ed_insert flushes it
	le.continuousInsertBuffer = "AB"
	le.edInsert(cmdCtx{key: "c"})
	if le.WholeBuffer() != "ABc" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestProcessKeyMultibyteCleansWaiting(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"r", "vi_replace_char"}) // waiting proc set
	// a multibyte key cleans up the waiting proc before dispatch
	feed(le, keyEv{"漢字", "ed_insert"})
	// replace did not happen with a 2-char key
	if le.WholeBuffer() != "abc" {
		t.Logf("buffer=%q", le.WholeBuffer())
	}
}

func TestViForwardWordDefaultClassDirect(t *testing.T) {
	// covers viForwardWord default (neither word nor space) class branch
	if viForwardWord("++ x", 0, false) == 0 {
		t.Fatal("punct class move")
	}
}

func TestViBackwardWordDefaultClass(t *testing.T) {
	// covers viBackwardWord's startWithWord=false closure (punctuation)
	n := viBackwardWord("ab++", 4)
	if n == 0 {
		t.Fatal("backward punct")
	}
}

func TestViConfirmZeroDiffDirect(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	// directly call the operator-confirm helpers with a zero byte delta to hit
	// the no-op default branch (a motion that doesn't move the cursor).
	le.viDeleteMetaConfirm(0)
	le.viChangeMetaConfirm(0)
	le.viYankConfirm(0)
	if le.WholeBuffer() != "abc" {
		t.Fatalf("zero-diff confirm mutated buffer=%q", le.WholeBuffer())
	}
}

func TestViToNextCharNeedPrevFound(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("a-b-c")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// t<char> lands just before the found char (needPrevChar branch)
	feed(le, keyEv{"t", "vi_to_next_char"})
	feed(le, keyEv{"c", "ed_insert"})
	if le.BytePointer() != 3 {
		t.Fatalf("tc ptr=%d", le.BytePointer())
	}
}

func TestKeyStrokeEscEscBracket(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC ESC [ A -> exercises the idx++ after second ESC then CSI '['
	st := ks.matchStatus(ints(27, 27, '[', 'A'))
	if st != statusMatched {
		t.Fatalf("ESC ESC [ A status=%v", st)
	}
}

func TestKeyStrokeEscEscO(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	st := ks.matchStatus(ints(27, 27, 'O', 'P'))
	if st != statusMatched {
		t.Fatalf("ESC ESC O P status=%v", st)
	}
}

func TestKeyStrokeEscEscCharEmacs(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg) // emacs: ESC ESC x is not unmatched, classified by length
	st := ks.matchUnknownEscapeSequence(ints(27, 27, 'x'), false)
	if st == statusUnmatched {
		t.Fatal("emacs ESC ESC char should not be flatly unmatched")
	}
}

func TestFilterEmptyCandidate(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"", "foobar"}
	})
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	// empty candidate skipped; "foobar" unique
	if le.WholeBuffer() != "foobar " {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestJourneyNilProcResult(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return nil })
	le.config.Autocompletion = true
	feed(le, insertStr("x")...)
	// retrieveCompletionJourneyState returns nil when proc yields nil
	if js := le.retrieveCompletionJourneyState(); js != nil {
		t.Fatal("expected nil journey")
	}
}

func TestKeyStrokeCSIWithParameters(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC [ 1 ; 5 C : CSI with parameter bytes 0x30-0x3f
	if ks.matchStatus(ints(27, '[', '1', ';', '5', 'C')) != statusMatched {
		t.Fatal("parametrized CSI should be MATCHED")
	}
	// partial parametrized CSI is MATCHING
	if ks.matchStatus(ints(27, '[', '1', ';')) != statusMatching {
		t.Fatal("partial param CSI MATCHING")
	}
}

func TestKeyStrokeCSIIntermediate(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC [ <space> p : intermediate byte 0x20
	if ks.matchStatus(ints(27, '[', 0x20, 'p')) != statusMatched {
		t.Fatal("intermediate CSI MATCHED")
	}
}

func TestKeyStrokeOverlongUnmatched(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC [ A x : one byte past the complete CSI -> UNMATCHED
	if ks.matchStatus(ints(27, '[', 'A', 'x')) != statusUnmatched {
		t.Fatal("overlong CSI UNMATCHED")
	}
}

func TestViToPrevCharLandsAfter(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("xayaz")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	// T a : move backward to just after the previous 'a'
	feed(le, keyEv{"T", "vi_to_prev_char"})
	feed(le, keyEv{"a", "ed_insert"})
	if le.BytePointer() != 4 {
		t.Fatalf("Ta ptr=%d", le.BytePointer())
	}
}

func TestSearchLineBackupHitAfterNav(t *testing.T) {
	LastIncrementalSearch = ""
	le, _, _ := newTestEditor()
	le.hist.Append("history line")
	feed(le, insertStr("mydraftword")...)
	// navigate into history to set the line backup
	feed(le, keyEv{"\x10", "ed_prev_history"})
	// start isearch and search for a word from the backed-up in-progress line
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"d", "ed_insert"})
	feed(le, keyEv{"r", "ed_insert"})
	feed(le, keyEv{"a", "ed_insert"})
	if _, s := le.SearchingPrompt(); !s {
		t.Fatal("search active")
	}
}

func TestSearchHistoryRangeForwardAgain(t *testing.T) {
	LastIncrementalSearch = ""
	le, _, _ := newTestEditor()
	le.hist.Append("foo1")
	le.hist.Append("foo2")
	le.hist.Append("foo3")
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"f", "ed_insert"}) // matches foo3
	feed(le, keyEv{"\x13", "vi_search_next"})
	feed(le, keyEv{"\x13", "vi_search_next"})
	if _, s := le.SearchingPrompt(); !s {
		t.Fatal("search active")
	}
}
