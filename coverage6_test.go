package reline

import "testing"

func TestViToPrevCharPrevTotalBranch(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abcXdef")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	// Tb: search backward for 'b' (several chars before), needNextChar branch
	// uses prevTotal (the char after the match).
	feed(le, keyEv{"T", "vi_to_prev_char"})
	feed(le, keyEv{"b", "ed_insert"})
	if le.BytePointer() != 2 {
		t.Fatalf("Tb ptr=%d", le.BytePointer())
	}
}

func TestSearchPrevHistoryNoPointerEmptyOnNonEmptyLine(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("xyz")
	feed(le, insertStr("foo")...)
	// not in history, with a substr from a non-empty current line that has no
	// match -> the function reaches search and returns without a hit.
	feed(le, keyEv{"p", "ed_search_prev_history"})
	if le.WholeBuffer() != "foo" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestSearchNextHistoryRestoreNoHit(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("apple")
	le.hist.Append("apricot")
	feed(le, insertStr("ap")...)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // -> apricot
	feed(le, keyEv{"p", "ed_search_prev_history"}) // -> apple (pointer 0)
	// move cursor so substr is empty, then forward search-next with empty substr
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"n", "ed_search_next_history"}) // empty-substr forward path
	if le.WholeBuffer() == "" {
		t.Fatal("forward navigated to empty")
	}
}

func TestKeyStrokeExpandEmptyInput(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// Empty input: the scan loop never runs, so no bytes match -> empty result.
	keys, rest := ks.expand(nil)
	if len(keys) != 0 {
		t.Fatalf("expected no keys, got %+v", keys)
	}
	if rest != nil {
		t.Fatalf("expected nil rest, got %v", rest)
	}
}

func TestViOperatorWithArgMotion(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("aa bb cc dd")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// d2w -> delete with arg 2 applied to the motion inside the operator path
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"w", "vi_next_word"})
	if le.WholeBuffer() != "cc dd" {
		t.Fatalf("d2w=%q", le.WholeBuffer())
	}
}

func TestSearchAgainReverseNoPointer(t *testing.T) {
	LastIncrementalSearch = "foo"
	le, _, _ := newTestEditor()
	le.hist.Append("foobar")
	le.hist.Append("foobaz")
	// start reverse search and immediately repeat (search-again) with empty word
	// and no history pointer -> searchHistoryRange returns (0, all).
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"\x12", "vi_search_prev"})
	if _, s := le.SearchingPrompt(); !s {
		t.Fatal("search active")
	}
}

func TestSearchPrevHistoryEmptySubstrCursorAtBol(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("apple")
	feed(le, insertStr("draft")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"}) // cursor at col 0 -> substr ""
	// not in history, empty substr, but current line non-empty -> early return
	feed(le, keyEv{"p", "ed_search_prev_history"})
	if le.WholeBuffer() != "draft" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestSearchNextHistoryNoMatchNonEmptySubstr(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("apple")
	le.hist.Append("apricot")
	le.hist.Append("banana")
	feed(le, insertStr("a")...)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // into history (apricot)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // apple
	// now forward with a substr that has no forward match -> return
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	feed(le, insertStr("zzz")...) // substr "applezzz" no match forward
	feed(le, keyEv{"n", "ed_search_next_history"})
	if le.WholeBuffer() == "" {
		t.Fatal("should not blank on no forward match")
	}
}

func TestSearchNextHistoryRestoreBackupEmptySubstr(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("aaa")
	le.hist.Append("bbb")
	// Start on an empty line so empty-substr prev-search navigates history (and
	// the in-progress empty line is the backup).
	feed(le, keyEv{"p", "ed_search_prev_history"}) // -> bbb (newest, pointer 1)
	if le.WholeBuffer() != "bbb" {
		t.Fatalf("into history=%q", le.WholeBuffer())
	}
	// forward empty-substr search from the newest entry: ascending range is
	// empty, so there is no hit and the in-progress (empty) backup is restored.
	feed(le, keyEv{"n", "ed_search_next_history"})
	if le.WholeBuffer() != "" {
		t.Fatalf("restore backup=%q", le.WholeBuffer())
	}
}

func TestTransposeWordsCursorOnSeparator(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("aa  bb")...)
	// place cursor on the separating spaces (non-word run)
	le.bytePointer = 3
	feed(le, keyEv{"t", "ed_transpose_words"})
	if le.WholeBuffer() != "bb  aa" {
		t.Fatalf("transpose on sep=%q", le.WholeBuffer())
	}
}
