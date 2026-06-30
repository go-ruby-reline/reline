package reline

import "testing"

func TestNullIOAll(t *testing.T) {
	n := &NullIO{}
	if n.GetC(0) != -1 {
		t.Fatal("getc empty")
	}
	n.UngetC('x')
	if n.GetC(0) != 'x' {
		t.Fatal("getc after unget")
	}
	r, c := n.GetScreenSize()
	if r != 24 || c != 80 {
		t.Fatalf("size=%d,%d", r, c)
	}
	n.Write("out")
	if len(n.Out) != 1 || n.Out[0] != "out" {
		t.Fatal("write")
	}
	n.MoveCursorColumn(5)
	if n.Col != 5 {
		t.Fatal("move col")
	}
	n.EraseAfterCursor()
	n.ClearScreen()
	if n.Cleared != 1 {
		t.Fatal("clear count")
	}
	n2 := &NullIO{Rows: 10, Cols: 40}
	if r, c := n2.GetScreenSize(); r != 10 || c != 40 {
		t.Fatalf("custom size=%d,%d", r, c)
	}
}

func TestEditorAccessors(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOff()
	le.SetBytePointer(0)
	if le.History() == nil {
		t.Fatal("history nil")
	}
	feed(le, insertStr("abc")...)
	le.SetBytePointer(1)
	if le.BytePointer() != 1 {
		t.Fatal("set byte pointer")
	}
}

func TestDeleteText(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello world")...)
	le.DeleteText(0, 6, true) // remove "hello "
	if le.CurrentLine() != "world" {
		t.Fatalf("delete range=%q", le.CurrentLine())
	}
	// no-args delete on single line clears it
	le.DeleteText(0, 0, false)
	if le.CurrentLine() != "" {
		t.Fatalf("delete all=%q", le.CurrentLine())
	}
}

func TestDeleteTextMultiline(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("b")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("c")...)
	// on last line: no-arg delete pops it
	le.DeleteText(0, 0, false)
	if len(le.WholeLines()) != 2 {
		t.Fatalf("after pop=%d", len(le.WholeLines()))
	}
	// move to middle line and delete it
	le.lineIndex = 0
	le.DeleteText(0, 0, false)
	if len(le.WholeLines()) != 1 {
		t.Fatalf("after mid delete=%d", len(le.WholeLines()))
	}
}

func TestRenderArgPrompt(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"3", "ed_argument_digit"})
	rs := le.Render(80, nil)
	if rs.Lines[0][0] != "(arg: 3) " {
		t.Fatalf("arg prompt=%q", rs.Lines[0][0])
	}
}

func TestRenderSearchPrompt(t *testing.T) {
	le := setupSearchEditor()
	feed(le, keyEv{"\x12", "vi_search_prev"})
	feed(le, keyEv{"a", "ed_insert"})
	rs := le.Render(80, nil)
	if rs.Lines[0][0] == "> " {
		t.Fatalf("expected search prompt, got %q", rs.Lines[0][0])
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 5: "5", 42: "42", -7: "-7", 100: "100"}
	for n, want := range cases {
		if got := itoa(n); got != want {
			t.Errorf("itoa(%d)=%q want %q", n, got, want)
		}
	}
}

func TestEdNextCharMultilineEmacs(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("cd")...)
	le.lineIndex = 0
	le.bytePointer = len(le.CurrentLine())
	// at end of line 0, next char moves to start of line 1
	feed(le, keyEv{"\x06", "ed_next_char"})
	if le.LineIndex() != 1 || le.BytePointer() != 0 {
		t.Fatalf("emacs next char across line: idx=%d ptr=%d", le.LineIndex(), le.BytePointer())
	}
	// at start of line 1, prev char moves to end of line 0
	feed(le, keyEv{"\x02", "ed_prev_char"})
	if le.LineIndex() != 0 {
		t.Fatalf("emacs prev char across line: idx=%d", le.LineIndex())
	}
}

func TestEmDeletePrevCharJoinsLines(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("cd")...)
	le.bytePointer = 0
	// backspace at start of line 1 joins with line 0
	feed(le, keyEv{"\x08", "em_delete_prev_char"})
	if le.WholeBuffer() != "abcd" {
		t.Fatalf("join=%q", le.WholeBuffer())
	}
}

func TestEdKillLineJoinsNext(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("cd")...)
	le.lineIndex = 0
	le.bytePointer = len(le.CurrentLine())
	// kill-line at EOL joins next line
	feed(le, keyEv{"\x0b", "ed_kill_line"})
	if le.WholeBuffer() != "abcd" {
		t.Fatalf("kill-join=%q", le.WholeBuffer())
	}
}

func TestEmDeleteJoinsNext(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("cd")...)
	le.lineIndex = 0
	le.bytePointer = len(le.CurrentLine())
	feed(le, keyEv{"\x04", "em_delete"})
	if le.WholeBuffer() != "abcd" {
		t.Fatalf("delete-join=%q", le.WholeBuffer())
	}
}

func TestViDeletePrevCharJoinsLines(t *testing.T) {
	le := newViEditor()
	le.MultilineOn()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("cd")...)
	le.bytePointer = 0
	feed(le, keyEv{"\x7f", "vi_delete_prev_char"})
	if le.WholeBuffer() != "abcd" {
		t.Fatalf("vi join=%q", le.WholeBuffer())
	}
}

func TestMultibyteWordMovement(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, keyEv{"漢", "ed_insert"}, keyEv{"字", "ed_insert"}, keyEv{" ", "ed_insert"})
	feed(le, keyEv{"a", "ed_insert"}, keyEv{"b", "ed_insert"})
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"f", "em_next_word"})
	// past "漢字" = 6 bytes
	if le.BytePointer() != 6 {
		t.Fatalf("mb word ptr=%d", le.BytePointer())
	}
}

func TestArgRepeatForwardChar(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("abcdef")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	// In emacs the numeric argument arrives via ed_argument_digit (ESC-digit).
	feed(le, keyEv{"3", "ed_argument_digit"})
	feed(le, keyEv{"\x06", "ed_next_char"})
	if le.BytePointer() != 3 {
		t.Fatalf("3 forward ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"\x02", "ed_prev_char"})
	if le.BytePointer() != 1 {
		t.Fatalf("2 back ptr=%d", le.BytePointer())
	}
}

func TestEdDigitInserts(t *testing.T) {
	le, _, _ := newTestEditor()
	// in emacs without arg, ed_digit inserts the digit
	feed(le, keyEv{"5", "ed_digit"})
	if le.WholeBuffer() != "5" {
		t.Fatalf("digit insert=%q", le.WholeBuffer())
	}
}

func TestViZeroAsArg(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world foo")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"}) // move to col 0
	feed(le, keyEv{"2", "ed_argument_digit"})
	feed(le, keyEv{"0", "vi_zero"}) // 0 after digit -> appends to arg => 20
	feed(le, keyEv{"l", "ed_next_char"})
	// arg 20 forward but line is shorter; clamps at end-1 in vi
	if le.BytePointer() == 0 {
		t.Fatal("vi 20l should move")
	}
}

func TestInsertRawChar(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, keyEv{"a", "insert_raw_char"})
	feed(le, keyEv{"\n", "insert_raw_char"})   // raw newline -> key_newline
	feed(le, keyEv{"\x00", "insert_raw_char"}) // NUL ignored
	if le.WholeBuffer() != "a\n" {
		t.Fatalf("raw=%q", le.WholeBuffer())
	}
}

func TestEdForceSubmit(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("x")...)
	feed(le, keyEv{"x", "ed_force_submit"})
	if !le.Finished() {
		t.Fatal("force submit")
	}
}

func TestClearScreen(t *testing.T) {
	le, _, io := newTestEditor()
	feed(le, keyEv{"\x0c", "ed_clear_screen"})
	_ = io
}

func TestTransposeWordsNoOp(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("foo")...) // single word -> no transpose
	feed(le, keyEv{"t", "ed_transpose_words"})
	if le.WholeBuffer() != "foo" {
		t.Fatalf("transpose no-op=%q", le.WholeBuffer())
	}
}

func TestExchangeMarkNoMark(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("abc")...)
	// no mark set -> no-op
	feed(le, keyEv{"x", "em_exchange_mark"})
	if le.BytePointer() != 3 {
		t.Fatalf("exchange no mark ptr=%d", le.BytePointer())
	}
}

func TestUndoRedoBoundaries(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("a")...)
	// redo at top boundary -> no change
	feed(le, keyEv{"\x12", "redo"})
	// undo past bottom
	feed(le, keyEv{"\x1f", "undo"})
	feed(le, keyEv{"\x1f", "undo"})
	feed(le, keyEv{"\x1f", "undo"})
	if le.WholeBuffer() != "" {
		t.Fatalf("undo to empty=%q", le.WholeBuffer())
	}
}
