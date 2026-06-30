package reline

import "testing"

func newCompletingEditor(proc CompletionProc) *LineEditor {
	cfg := NewConfig()
	le := NewLineEditor(cfg, &NullIO{}, NewHistory(-1))
	le.Reset("> ")
	le.SetCompletionProc(proc)
	le.SetCompletionAppendCharacter(" ")
	return le
}

func TestCompletionUniquePrefix(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foobar"}
	})
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	// unique candidate -> completed + append char
	if le.WholeBuffer() != "foobar " {
		t.Fatalf("complete=%q", le.WholeBuffer())
	}
}

func TestCompletionCommonPrefix(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foobar", "foobaz"}
	})
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	// common prefix "fooba", no append char (ambiguous)
	if le.WholeBuffer() != "fooba" {
		t.Fatalf("complete=%q", le.WholeBuffer())
	}
}

func TestCompletionMenuState(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foobar", "foobaz"}
	})
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"}) // -> common prefix "fooba", state MENU
	feed(le, keyEv{"\t", "complete"}) // -> menu shown
	menu, ok := le.MenuInfo()
	if !ok || len(menu) != 2 {
		t.Fatalf("menu=%v ok=%v", menu, ok)
	}
}

func TestCompletionPerfectMatchMenu(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foo", "foobar"}
	})
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"}) // "foo" is in candidates, multiple -> MENU_WITH_PERFECT_MATCH
	feed(le, keyEv{"\t", "complete"}) // shows menu, state PERFECT_MATCH
	menu, ok := le.MenuInfo()
	if !ok || len(menu) != 2 {
		t.Fatalf("menu=%v", menu)
	}
}

func TestCompletionShowAllIfAmbiguous(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foobar", "foobaz"}
	})
	le.config.ShowAllIfAmbiguous = true
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	if _, ok := le.MenuInfo(); !ok {
		t.Fatal("expected immediate menu")
	}
}

func TestCompletionPerfectMatchSingleAppend(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"unique"}
	})
	le.config.ShowAllIfAmbiguous = true
	feed(le, insertStr("uni")...)
	feed(le, keyEv{"\t", "complete"})
	if le.WholeBuffer() != "unique " {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestCompletionNoCandidates(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"xyz"}
	})
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	// no candidate starts with "foo" -> unchanged
	if le.WholeBuffer() != "foo" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestCompletionNilProcResult(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return nil })
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	if le.WholeBuffer() != "foo" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestCompletionDisabled(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return []string{"foobar"} })
	le.config.DisableCompletion = true
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	if le.WholeBuffer() != "foo" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestCompletionIgnoreCase(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return []string{"FooBar"} })
	le.config.CompletionIgnoreCase = true
	le.completionIgnoreCase = true
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\t", "complete"})
	if le.WholeBuffer() != "FooBar " {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestAutocompletionJourney(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"alpha", "alpine", "alps"}
	})
	le.config.Autocompletion = true
	feed(le, insertStr("al")...)
	feed(le, keyEv{"\t", "menu_complete"})
	// cycles to first candidate
	if le.WholeBuffer() != "alpha" {
		t.Fatalf("journey down=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\t", "menu_complete"})
	if le.WholeBuffer() != "alpine" {
		t.Fatalf("journey down2=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"Z", "menu_complete_backward"})
	if le.WholeBuffer() != "alpha" {
		t.Fatalf("journey up=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"U", "completion_journey_up"})
	if le.WholeBuffer() != "al" {
		t.Fatalf("journey up to target=%q", le.WholeBuffer())
	}
}

func TestCompleteViaTabAutocomplete(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"xx", "xy"}
	})
	le.config.Autocompletion = true
	feed(le, insertStr("x")...)
	feed(le, keyEv{"\t", "complete"})
	if le.WholeBuffer() != "xx" {
		t.Fatalf("autocomplete tab=%q", le.WholeBuffer())
	}
}

func TestRetrieveCompletionBlockQuote(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string { return nil })
	feed(le, insertStr("foo \"bar")...)
	pre, target, _, quote := le.retrieveCompletionBlock()
	if target != "bar" {
		t.Fatalf("target=%q", target)
	}
	if quote != "\"" {
		t.Fatalf("quote=%q", quote)
	}
	if pre != "foo \"" {
		t.Fatalf("pre=%q", pre)
	}
}

func TestEmDeleteOrListMenu(t *testing.T) {
	le := newCompletingEditor(func(target, pre, post string) []string {
		return []string{"foobar", "foobaz"}
	})
	feed(le, insertStr("foo")...)
	// cursor at end, non-empty: em_delete_or_list shows candidate list
	feed(le, keyEv{"\x04", "em_delete_or_list"})
	if menu, ok := le.MenuInfo(); !ok || len(menu) != 2 {
		t.Fatalf("delete_or_list menu=%v", menu)
	}
}
