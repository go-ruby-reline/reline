package reline

import "testing"

// These tests target the remaining edge branches to keep coverage at 100%.

func TestEdIgnore(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\x03", "ed_ignore"}) // Ctrl-C is ed_ignore in emacs
	if le.WholeBuffer() != "ab" {
		t.Fatalf("ignore changed buffer=%q", le.WholeBuffer())
	}
}

func TestViSearchNextDirect(t *testing.T) {
	le := setupSearchEditor()
	// Start a forward search directly (not as a repeat).
	feed(le, keyEv{"\x13", "vi_search_next"})
	if _, searching := le.SearchingPrompt(); !searching {
		t.Fatal("forward search should start")
	}
	feed(le, keyEv{"c", "ed_insert"})
	feed(le, keyEv{"\x01", "ed_move_to_beg"}) // terminate
}

func TestEdInsertInPasting(t *testing.T) {
	le, _, _ := newTestEditor()
	le.SetPastingState(true)
	// ed_insert during pasting buffers; a non-insert command forces flush
	feed(le, keyEv{"a", "ed_insert"})
	feed(le, keyEv{"b", "ed_insert"})
	// force flush via a movement command (processInsert force)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	if le.WholeBuffer() != "ab" {
		t.Fatalf("paste flush=%q", le.WholeBuffer())
	}
	le.SetPastingState(false)
}

func TestEdInsertFlushesContinuous(t *testing.T) {
	le, _, _ := newTestEditor()
	le.SetPastingState(true)
	feed(le, keyEv{"x", "ed_insert"})
	le.SetPastingState(false) // flush "x"
	feed(le, keyEv{"y", "ed_insert"})
	if le.WholeBuffer() != "xy" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestViChangeMetaWithArg(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("aaa bbb ccc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// 2cw -> change 2 words
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"c", "vi_change_meta"})
	feed(le, keyEv{"w", "vi_next_word"})
	// matches MRI: 2cw with drop_terminate_spaces deletes through the 2nd word
	// start, leaving "bbb ccc".
	if le.WholeBuffer() != "bbb ccc" {
		t.Fatalf("2cw=%q", le.WholeBuffer())
	}
}

func TestViYankWithArgAndBackward(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	// yb -> yank backward word
	feed(le, keyEv{"y", "vi_yank"})
	feed(le, keyEv{"b", "vi_prev_word"})
	feed(le, keyEv{"p", "vi_paste_next"})
	if le.WholeBuffer() == "hello world" {
		t.Fatalf("yb-p did nothing: %q", le.WholeBuffer())
	}
}

func TestViDeleteMetaBackward(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	// db -> delete backward word
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"b", "vi_prev_word"})
	if le.WholeBuffer() != "hello d" {
		t.Fatalf("db=%q", le.WholeBuffer())
	}
}

func TestViYankConfirmZeroDiff(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// yank with a motion that doesn't move (0 at col 0)
	feed(le, keyEv{"y", "vi_yank"})
	feed(le, keyEv{"0", "vi_zero"})
	// no clipboard change but should not crash
	if le.WholeBuffer() != "abc" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestEdDeleteNextCharAtEnd(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	// cursor at end (vi command moved back to 'b'); delete it
	feed(le, keyEv{"x", "ed_delete_next_char"})
	if le.WholeBuffer() != "a" {
		t.Fatalf("x at end=%q", le.WholeBuffer())
	}
}

func TestViEndWordArgAndInclusive(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar baz")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"e", "vi_end_word"})
	// 2e -> end of second word "bar"
	if le.BytePointer() != 6 {
		t.Fatalf("2e ptr=%d", le.BytePointer())
	}
}

func TestViBigEndWordArg(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("a.b c.d e.f")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"E", "vi_end_big_word"})
	if le.BytePointer() != 6 {
		t.Fatalf("2E ptr=%d", le.BytePointer())
	}
}

func TestInsertMultilineTextEmpty(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	le.InsertMultilineText("")
	if le.WholeBuffer() != "" {
		t.Fatalf("empty insert=%q", le.WholeBuffer())
	}
}

func TestPushUndoNonModifying(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("a")...)
	// movement does not modify -> updates current undo slot
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	if le.WholeBuffer() != "a" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestUndoHistoryCap(t *testing.T) {
	le, _, _ := newTestEditor()
	for i := 0; i < 150; i++ {
		feed(le, keyEv{"x", "ed_insert"})
	}
	if len(le.undoRedoHistory) > maxUndoRedoHistorySize {
		t.Fatalf("undo history not capped: %d", len(le.undoRedoHistory))
	}
}

func TestCompletionCallProcNilTarget(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return []string{"x"} })
	// empty target (cursor at start) -> proc not called, returns nil
	if got := le.callCompletionProc("", "", ""); got != nil {
		t.Fatalf("expected nil for empty target, got %v", got)
	}
}

func TestCompletionMenuComplete(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"abc", "abd"}
	})
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\t", "menu_complete"})
	if le.WholeBuffer() != "abc" {
		t.Fatalf("menu_complete=%q", le.WholeBuffer())
	}
}

func TestCompletionJourneyDisabled(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return []string{"x"} })
	le.config.DisableCompletion = true
	feed(le, insertStr("x")...)
	feed(le, keyEv{"\t", "menu_complete"})
	// disabled -> no journey
	if le.WholeBuffer() != "x" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestCompletionJourneyNoCandidates(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return []string{"zzz"} })
	le.config.Autocompletion = true
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\t", "menu_complete"})
	if le.WholeBuffer() != "a" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestViForwardWordOnSymbols(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("a.b.c d")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"w", "vi_next_word"}) // from 'a' to '.'
	if le.BytePointer() != 1 {
		t.Fatalf("vi w symbol ptr=%d", le.BytePointer())
	}
}

func TestViForwardEndWordSingleChar(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("a bc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"e", "vi_end_word"})
	// from single-char word 'a', e goes to end of next word "bc"
	if le.BytePointer() != 3 {
		t.Fatalf("vi e single=%d", le.BytePointer())
	}
}

func TestViBackwardWordSymbols(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo...bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	feed(le, keyEv{"b", "vi_prev_word"}) // back over "bar"
	if le.BytePointer() != 6 {
		t.Fatalf("vi b symbol=%d", le.BytePointer())
	}
}

func TestEmptyLineGraphemeBranches(t *testing.T) {
	// exercise getNext/getPrev on empty + edge offsets
	if getNextMbcharSize("", 0) != 0 {
		t.Fatal("next empty")
	}
	if getPrevMbcharSize("", 0) != 0 {
		t.Fatal("prev empty")
	}
}

func TestCommonPrefixSingleItem(t *testing.T) {
	if got := CommonPrefix([]string{"only"}, false); got != "only" {
		t.Fatalf("single=%q", got)
	}
}

func TestScanWidthLoneEsc(t *testing.T) {
	// lone ESC not forming CSI/OSC is treated as a grapheme
	w := CalculateWidth("\x1bx", true)
	if w < 1 {
		t.Fatalf("lone esc width=%d", w)
	}
	// incomplete OSC (no terminator)
	CalculateWidth("\x1b]0;abc", true)
	// incomplete CSI
	CalculateWidth("\x1b[", true)
}

func TestMatchOSCNoDigit(t *testing.T) {
	// ESC ] with no digit -> not an OSC
	w := CalculateWidth("\x1b]abc\x07", true)
	if w == 0 {
		t.Fatalf("non-osc width=%d", w)
	}
}

func TestFilterDuplicates(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foo", "foo", "foobar"}
	})
	feed(le, insertStr("f")...)
	feed(le, keyEv{"\t", "complete"})
	// "foo" duplicate filtered; common prefix "foo"
	if le.WholeBuffer() != "foo" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestEmExchangeMarkRoundTrip(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"\x00", "em_set_mark"})
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	feed(le, keyEv{"x", "em_exchange_mark"})
	if le.BytePointer() != 0 {
		t.Fatalf("first exchange=%d", le.BytePointer())
	}
	feed(le, keyEv{"x", "em_exchange_mark"})
	if le.BytePointer() != 5 {
		t.Fatalf("second exchange=%d", le.BytePointer())
	}
}

func TestViPastePrevMultichar(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// delete two chars to clipboard via 2x? Use d then l motion
	feed(le, keyEv{"y", "vi_yank"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	// clipboard = "hell" (yank to end-ish); paste before
	feed(le, keyEv{"P", "vi_paste_prev"})
	if le.WholeBuffer() == "hello" {
		t.Fatalf("paste prev did nothing")
	}
}

func TestLowerCaseWithUppercase(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("HELLO world")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"l", "em_lower_case"})
	if le.WholeBuffer() != "hello world" {
		t.Fatalf("lower=%q", le.WholeBuffer())
	}
}

func TestSearchHistoryAcrossMultilineEntry(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("def foo\n  bar")
	le.hist.Append("class X")
	feed(le, insertStr("  ")...)
	feed(le, keyEv{"p", "ed_search_prev_history"})
	// "  " prefix matches the second line "  bar" of the first entry
	if le.LineIndex() != 1 {
		t.Fatalf("multiline search line=%d buf=%q", le.LineIndex(), le.WholeBuffer())
	}
}
