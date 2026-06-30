package reline

import "testing"

func TestHistoryPushAndCap(t *testing.T) {
	h := NewHistory(3)
	h.Push("a", "b")
	h.Push("c", "d") // exceeds cap -> drop oldest
	if h.Size() != 3 {
		t.Fatalf("size=%d", h.Size())
	}
	got, _ := h.Get(0)
	if got != "b" {
		t.Fatalf("get0=%q", got)
	}
	got2, _ := h.Get(-1)
	if got2 != "d" {
		t.Fatalf("get-1=%q", got2)
	}
}

func TestHistoryPushMoreThanCap(t *testing.T) {
	h := NewHistory(2)
	h.Push("a", "b", "c", "d") // 4 vals into cap 2 -> keep last 2
	if h.Size() != 2 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "c" {
		t.Fatalf("get0=%q", v)
	}
}

func TestHistoryPushExactlyEmptiesThenShrinks(t *testing.T) {
	h := NewHistory(3)
	h.Push("a", "b", "c")
	h.Push("d", "e", "f", "g") // diff = 4 > size(3): clear then drop 1 from vals
	if h.Size() != 3 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "e" {
		t.Fatalf("get0=%q", v)
	}
}

func TestHistoryZeroAndNegativeSize(t *testing.T) {
	z := NewHistory(0)
	z.Push("x")
	z.Append("y")
	if z.Size() != 0 {
		t.Fatalf("zero size=%d", z.Size())
	}
	n := NewHistory(-1)
	for i := 0; i < 100; i++ {
		n.Append("v")
	}
	if n.Size() != 100 {
		t.Fatalf("unlimited size=%d", n.Size())
	}
}

func TestHistoryAppendCap(t *testing.T) {
	h := NewHistory(2)
	h.Append("a")
	h.Append("b")
	h.Append("c")
	if h.Size() != 2 {
		t.Fatalf("size=%d", h.Size())
	}
	v, _ := h.Get(0)
	if v != "b" {
		t.Fatalf("get0=%q", v)
	}
}

func TestHistorySetDeleteClear(t *testing.T) {
	h := NewHistory(-1)
	h.Push("a", "b", "c")
	if err := h.Set(1, "B"); err != nil {
		t.Fatal(err)
	}
	v, _ := h.Get(1)
	if v != "B" {
		t.Fatalf("set=%q", v)
	}
	d, err := h.DeleteAt(0)
	if err != nil || d != "a" {
		t.Fatalf("delete=%q err=%v", d, err)
	}
	if h.Size() != 2 {
		t.Fatalf("after delete size=%d", h.Size())
	}
	h.Clear()
	if !h.Empty() {
		t.Fatal("not empty after clear")
	}
}

func TestHistoryIndexErrors(t *testing.T) {
	h := NewHistory(-1)
	h.Push("a")
	if _, err := h.Get(5); err == nil {
		t.Fatal("expected out-of-bounds error")
	}
	if _, err := h.Get(-5); err == nil {
		t.Fatal("expected negative out-of-bounds error")
	}
	if err := h.Set(9, "x"); err == nil {
		t.Fatal("expected set error")
	}
	if _, err := h.DeleteAt(9); err == nil {
		t.Fatal("expected delete error")
	}
	// too-big integer
	if _, err := h.Get(3000000000); err == nil {
		t.Fatal("expected range error")
	}
	if err, ok := errCheck(h.Set(3000000000, "x")); !ok {
		t.Fatalf("expected HistoryError, got %v", err)
	}
}

func TestHistorySizeCapBounds(t *testing.T) {
	h := NewHistory(2)
	h.Push("a", "b")
	// index beyond cap raises
	if _, err := h.Get(2); err == nil {
		t.Fatal("expected cap range error")
	}
}

func errCheck(err error) (error, bool) {
	_, ok := err.(*HistoryError)
	return err, ok
}

func TestHistoryErrorMessage(t *testing.T) {
	e := &HistoryError{msg: "boom"}
	if e.Error() != "boom" {
		t.Fatalf("msg=%q", e.Error())
	}
}

func TestHistoryNavigationUpDown(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("one")
	le.hist.Append("two")
	le.hist.Append("three")
	feed(le, insertStr("draft")...)
	feed(le, keyEv{"\x10", "ed_prev_history"}) // up -> three
	if le.WholeBuffer() != "three" {
		t.Fatalf("up1=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\x10", "ed_prev_history"}) // up -> two
	if le.WholeBuffer() != "two" {
		t.Fatalf("up2=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\x0e", "ed_next_history"}) // down -> three
	if le.WholeBuffer() != "three" {
		t.Fatalf("down=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\x0e", "ed_next_history"}) // down -> back to draft
	if le.WholeBuffer() != "draft" {
		t.Fatalf("restore draft=%q", le.WholeBuffer())
	}
}

func TestHistoryBeginningEnd(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("a")
	le.hist.Append("b")
	le.hist.Append("c")
	feed(le, keyEv{"x", "ed_beginning_of_history"})
	if le.WholeBuffer() != "a" {
		t.Fatalf("beg=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"x", "ed_end_of_history"})
	// end_of_history goes to HISTORY.size (the in-progress backup, "")
	if le.WholeBuffer() != "" {
		t.Fatalf("end=%q", le.WholeBuffer())
	}
}

func TestHistoryPrefixSearch(t *testing.T) {
	le, _, _ := newTestEditor()
	le.hist.Append("apple")
	le.hist.Append("banana")
	le.hist.Append("apricot")
	feed(le, insertStr("ap")...)
	feed(le, keyEv{"p", "ed_search_prev_history"}) // search backward for "ap*"
	if le.WholeBuffer() != "apricot" {
		t.Fatalf("prefix search=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"p", "ed_search_prev_history"})
	if le.WholeBuffer() != "apple" {
		t.Fatalf("prefix search2=%q", le.WholeBuffer())
	}
	feed(le, keyEv{"n", "ed_search_next_history"})
	if le.WholeBuffer() != "apricot" {
		t.Fatalf("next search=%q", le.WholeBuffer())
	}
}

func TestMultilineHistoryNavigation(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("b")...)
	// cursor on line 1; prev_history within buffer moves up a line
	feed(le, keyEv{"\x10", "ed_prev_history"})
	if le.LineIndex() != 0 {
		t.Fatalf("multiline up line=%d", le.LineIndex())
	}
	feed(le, keyEv{"\x0e", "ed_next_history"})
	if le.LineIndex() != 1 {
		t.Fatalf("multiline down line=%d", le.LineIndex())
	}
}
