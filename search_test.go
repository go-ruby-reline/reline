package reline

import "testing"

func setupSearchEditor() *LineEditor {
	le, _, _ := newTestEditor()
	le.hist.Append("apple pie")
	le.hist.Append("banana split")
	le.hist.Append("cherry cake")
	return le
}

func TestIncrementalReverseSearch(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	feed(le, keyEv{"\x12", "vi_search_prev"}) // start reverse i-search
	// type "an" -> should match "banana split"
	feed(le, keyEv{"a", "ed_insert"})
	feed(le, keyEv{"n", "ed_insert"})
	if le.WholeBuffer() != "banana split" {
		t.Fatalf("isearch=%q", le.WholeBuffer())
	}
	// terminate with a movement key
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	_, searching := le.SearchingPrompt()
	if searching {
		t.Fatal("search should have terminated")
	}
}

func TestIncrementalSearchBackspace(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"c", "ed_insert"}) // matches cherry
	feed(le, keyEv{"z", "ed_insert"}) // "cz" no match
	feed(le, keyEv{"\x7f", "em_delete_prev_char"})
	if le.WholeBuffer() != "cherry cake" {
		t.Fatalf("after backspace=%q", le.WholeBuffer())
	}
}

func TestIncrementalSearchCancel(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	feed(le, insertStr("draft")...)
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"})
	feed(le, keyEv{"\x07", "ed_ignore"}) // Ctrl-G cancel
	if le.WholeBuffer() != "draft" {
		t.Fatalf("cancel restore=%q", le.WholeBuffer())
	}
	if _, searching := le.SearchingPrompt(); searching {
		t.Fatal("search not cleared after cancel")
	}
}

func TestIncrementalSearchRepeat(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	le.hist.Append("apricot jam")
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"}) // matches most recent containing "a"
	first := le.WholeBuffer()
	feed(le, keyEv{"\x12", "vi_search_prev"}) // search again backward
	second := le.WholeBuffer()
	if first == second {
		t.Fatalf("repeat search did not advance: %q", first)
	}
}

func TestIncrementalForwardSearch(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	// go back in history first
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"})
	feed(le, keyEv{"\x13", "vi_search_next"}) // forward search-again
	if _, searching := le.SearchingPrompt(); !searching {
		t.Fatal("forward search should be active")
	}
	feed(le, keyEv{"\n", "ed_ignore"}) // terminate
}

func TestIncrementalSearchFailed(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"z", "ed_insert"})
	feed(le, keyEv{"z", "ed_insert"})
	feed(le, keyEv{"z", "ed_insert"})
	prompt, searching := le.SearchingPrompt()
	if !searching {
		t.Fatal("search active")
	}
	if prompt == "" {
		t.Fatal("expected failed prompt text")
	}
}

func TestIncrementalSearchLastTerm(t *testing.T) {
	LastIncrementalSearch = ""
	le := setupSearchEditor()
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"p", "ed_insert"})
	feed(le, keyEv{"\x01", "ed_move_to_beg"}) // terminate, sets last search
	if LastIncrementalSearch != "p" {
		t.Fatalf("last search=%q", LastIncrementalSearch)
	}
	// new search, repeat with empty word should reuse last term
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"\x12", "vi_search_prev"})
	if _, searching := le.SearchingPrompt(); !searching {
		t.Fatal("search active")
	}
}
