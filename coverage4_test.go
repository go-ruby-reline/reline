package reline

import "testing"

// Direct unit tests on unexported helpers to reach defensive/empty-input
// branches that the high-level command flow guards against.

func TestUnicodeWordHelpersEmpty(t *testing.T) {
	if emForwardWord("", 0) != 0 {
		t.Fatal("emForwardWord empty")
	}
	if viBigForwardEndWord("", 0) != 0 {
		t.Fatal("viBigForwardEndWord empty")
	}
	if viForwardWord("", 0, false) != 0 {
		t.Fatal("viForwardWord empty")
	}
	if viForwardEndWord("", 0) != 0 {
		t.Fatal("viForwardEndWord empty")
	}
	if viForwardEndWord("a", 0) != 1 {
		t.Fatal("viForwardEndWord single")
	}
}

func TestViForwardWordPunctClassDirect(t *testing.T) {
	// start on punctuation: takes the !word && !space class
	n := viForwardWord("..ab", 0, false)
	if n != 2 {
		t.Fatalf("punct class=%d", n)
	}
}

func TestViForwardEndWordPunctDirect(t *testing.T) {
	n := viForwardEndWord("a..b", 0)
	if n == 0 {
		t.Fatal("end word punct")
	}
}

func TestEdTransposeWordsCursorMidWord(t *testing.T) {
	// cursor inside the second word triggers the else branch
	lws, ms, rws, awe := edTransposeWords("aa bcd", 4) // cursor in "bcd"
	if !(lws <= ms && ms <= rws && rws <= awe) {
		t.Fatalf("offsets out of order: %d %d %d %d", lws, ms, rws, awe)
	}
}

func TestEdTransposeWordsCursorAfterTrailing(t *testing.T) {
	// cursor at end after trailing non-words triggers the pos==len branch
	lws, _, _, awe := edTransposeWords("aa bb  ", 7)
	if lws < 0 || awe < lws {
		t.Fatal("trailing offsets")
	}
}

func TestCommonPrefixCaseFoldBreak(t *testing.T) {
	// ignoreCase path where chars differ -> break
	if got := CommonPrefix([]string{"ABx", "aby"}, true); got != "AB" {
		t.Fatalf("casefold prefix=%q", got)
	}
}

func TestGetMbcharWidthZWJ(t *testing.T) {
	// width with ZWJ join inside a single mbchar string
	w := getMbcharWidth("a‍b")
	if w < 1 {
		t.Fatalf("zwj width=%d", w)
	}
}

func TestEastAsianWidthBeyondTable(t *testing.T) {
	// a codepoint beyond the last chunk clamps to the final width
	w := eastAsianWidth(0x10FFFF)
	if w < 0 {
		t.Fatalf("beyond table=%d", w)
	}
}

func TestSplitLineResetColor(t *testing.T) {
	// reset sequence "\e[0m" clears carried color state
	lines := SplitLineByWidth("\x1b[31mab\x1b[0mcd", 2, 0)
	if len(lines) < 2 {
		t.Fatalf("color reset split=%v", lines)
	}
}

func TestScannerEmitInvalidByte(t *testing.T) {
	var got []string
	scanWidth("\xff\xfe", func(_ tokenKind, s string) { got = append(got, s) })
	if len(got) == 0 {
		t.Fatal("invalid bytes should still emit")
	}
}

func TestMatchCSIWithIntermediate(t *testing.T) {
	// CSI with intermediate bytes (0x20-0x2f), e.g. ESC [ space p
	w := CalculateWidth("\x1b[ pX", true)
	if w != 1 {
		t.Fatalf("csi intermediate width=%d", w)
	}
}

func TestMatchOSCEmptySegment(t *testing.T) {
	// ESC ] 8 ;; ... has an empty segment -> not OSC, treated as graphemes
	w := CalculateWidth("\x1b]8;;x\x07", true)
	if w == 0 {
		t.Fatalf("empty osc seg width=%d", w)
	}
}

func TestKeyActorCompositeNilLayer(t *testing.T) {
	c := &compositeKeyActor{actors: []*keyActor{nil, newEmptyKeyActor()}}
	if c.matching(ints(1)) {
		t.Fatal("nil layer matching")
	}
	if _, ok := c.get(ints(1)); ok {
		t.Fatal("nil layer get")
	}
}

func TestKeyActorGetMiss(t *testing.T) {
	ka := newEmptyKeyActor()
	if _, ok := ka.get(ints(99)); ok {
		t.Fatal("empty get")
	}
}

func TestKeyStrokeMatchingOnlyStatus(t *testing.T) {
	cfg := NewConfig()
	// register a 3-byte binding so a 2-byte prefix is MATCHING-only
	cfg.additional[ModeEmacs].add(ints('q', 'w', 'e'), "ed_move_to_end")
	ks := newKeyStroke(cfg)
	if ks.matchStatus(ints('q', 'w')) != statusMatching {
		t.Fatalf("prefix should be MATCHING")
	}
}

func TestKeyStrokeExpandNoMatchAtAll(t *testing.T) {
	cfg := NewConfig()
	cfg.SetEditingMode(ModeViCommand)
	ks := newKeyStroke(cfg)
	// ESC + char in vi -> UNMATCHED on first byte combos, no decoded key
	keys, rest := ks.expand(ints(27, 'q'))
	_ = keys
	_ = rest
}

func TestKeyStrokeEscEscNoFinal(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC ESC [ : double-esc CSI prefix
	st := ks.matchStatus(ints(27, 27, '['))
	if st != statusMatching {
		t.Fatalf("ESC ESC [ status=%v", st)
	}
}

func TestHistoryNavMoveBackupRestore(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("one")
	feed(le, insertStr("draft")...)
	feed(le, keyEv{"\x10", "ed_prev_history"}) // -> one, backup "draft"
	if le.WholeBuffer() != "one" {
		t.Fatalf("up=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\x0e", "ed_next_history"}) // -> restore "draft"
	if le.WholeBuffer() != "draft" {
		t.Fatalf("restore=%q", le.WholeBuffer())
	}
}

func TestMoveHistoryOutOfRange(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("a")
	// moveHistory with explicit out-of-range pointer is a no-op
	le.moveHistory(99, true, lineStart, cursorStart, 0, 0)
	if le.WholeBuffer() != "" {
		t.Fatalf("oob move=%q", le.WholeBuffer())
	}
	le.moveHistory(-5, true, lineStart, cursorStart, 0, 0)
}

func TestMoveHistoryNilPointer(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("a")
	feed(le, insertStr("d")...)
	// move with hasHistoryPointer=false uses HISTORY.size
	le.moveHistory(0, false, lineEnd, cursorEnd, 0, 0)
}

func TestSearchPrevHistoryAtZero(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("apple")
	feed(le, insertStr("a")...)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // -> apple (pointer 0)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // already at 0 -> return
	if le.WholeBuffer() != "apple" {
		t.Fatalf("at-zero=%q", le.WholeBuffer())
	}
}

func TestSearchPrevHistoryNoHistoryNonEmpty(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("xyz")...)
	// no history pointer, substr empty? substr="xyz" non-empty, no history -> no hit returns
	feed(le, keyEv{"p", "ed_search_prev_history"})
	if le.WholeBuffer() != "xyz" {
		t.Fatalf("no-hist=%q", le.WholeBuffer())
	}
}

func TestSearchNextHistoryNoPointer(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("a")
	// next search with no history pointer -> return
	feed(le, keyEv{"n", "ed_search_next_history"})
	if le.WholeBuffer() != "" {
		t.Fatalf("next no-ptr=%q", le.WholeBuffer())
	}
}

func TestSearchNextHistoryRestoreBackup(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("apple")
	le.hist.Append("apricot")
	feed(le, insertStr("a")...)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // apricot
	feed(le, keyEv{"p", "ed_search_prev_history"}) // apple
	// now forward with empty-substr after navigation
	feed(le, keyEv{"n", "ed_search_next_history"})
	if le.WholeBuffer() == "" {
		t.Fatal("forward restore empty")
	}
}

func TestProcessKeyCleanupOnMultibyteWaiting(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"f", "vi_next_char"}) // sets waiting proc
	// a multibyte key (size>1) cleans up the waiting proc
	feed(le, keyEv{"漢", "ed_insert"})
	// waiting proc cleaned; buffer unchanged by the search
	if le.WholeBuffer() != "hello" {
		t.Fatalf("cleanup multibyte=%q", le.WholeBuffer())
	}
}

func TestRunForOperatorsEdInsertViCommand(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	// ed_insert in vi command mode with no waiting proc is rejected
	feed(le, keyEv{"z", "ed_insert"})
	if le.WholeBuffer() != "ab" {
		t.Fatalf("vi-cmd insert rejected=%q", le.WholeBuffer())
	}
}

func TestActivePromptSearch(t *testing.T) {
	le := setupSearchEditor()
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"})
	p := le.activePrompt()
	if p == "> " {
		t.Fatal("search active prompt")
	}
}
