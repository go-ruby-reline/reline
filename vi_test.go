package reline

import "testing"

func newViEditor() *LineEditor {
	cfg := NewConfig()
	cfg.SetEditingMode(ModeViInsert)
	io := &NullIO{}
	le := NewLineEditor(cfg, io, NewHistory(-1))
	le.Reset("> ")
	return le
}

func feedVi(le *LineEditor, evs ...keyEv) { feed(le, evs...) }

func TestViCommandModeMovement(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	// after esc cursor moves back one; go to col 0
	feed(le, keyEv{"0", "vi_zero"})
	if le.BytePointer() != 0 {
		t.Fatalf("vi 0 ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"w", "vi_next_word"})
	if le.BytePointer() != 6 {
		t.Fatalf("vi w ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"b", "vi_prev_word"})
	if le.BytePointer() != 0 {
		t.Fatalf("vi b ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"e", "vi_end_word"})
	if le.BytePointer() != 4 {
		t.Fatalf("vi e ptr=%d", le.BytePointer())
	}
}

func TestViBigWords(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo.bar baz.qux")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"W", "vi_next_big_word"})
	if le.BytePointer() != 8 {
		t.Fatalf("vi W ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"E", "vi_end_big_word"})
	if le.BytePointer() != 14 {
		t.Fatalf("vi E ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"B", "vi_prev_big_word"})
	if le.BytePointer() != 8 {
		t.Fatalf("vi B ptr=%d", le.BytePointer())
	}
}

func TestViDeleteAndPaste(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"x", "ed_delete_next_char"})
	if le.WholeBuffer() != "ello" {
		t.Fatalf("vi x =%q", le.WholeBuffer())
	}
	// paste the deleted char after cursor
	feed(le, keyEv{"p", "vi_paste_next"})
	if le.WholeBuffer() != "ehllo" {
		t.Fatalf("vi p =%q", le.WholeBuffer())
	}
	feed(le, keyEv{"P", "vi_paste_prev"})
	if le.WholeBuffer() != "ehhllo" {
		t.Fatalf("vi P =%q", le.WholeBuffer())
	}
}

func TestViDeletePrevChar(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	feed(le, keyEv{"\x7f", "vi_delete_prev_char"})
	if le.WholeBuffer() != "ab" {
		t.Fatalf("=%q", le.WholeBuffer())
	}
}

func TestViChangeMetaOperator(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// cw -> change word: c (operator) then w (motion)
	feed(le, keyEv{"c", "vi_change_meta"})
	feed(le, keyEv{"w", "vi_next_word"})
	// vi `cw` with drop_terminate_spaces leaves the trailing space (MRI).
	if le.WholeBuffer() != " bar" {
		t.Fatalf("cw =%q", le.WholeBuffer())
	}
	// should be back in insert mode
	if !le.config.EditingModeIs(ModeViInsert) {
		t.Fatal("cw should enter insert mode")
	}
}

func TestViDeleteMetaOperator(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"w", "vi_next_word"})
	if le.WholeBuffer() != "bar" {
		t.Fatalf("dw =%q", le.WholeBuffer())
	}
}

func TestViDeleteMetaDoubled(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	// dd: operator then operator -> delete whole line
	feed(le, keyEv{"d", "vi_delete_meta"})
	feed(le, keyEv{"d", "vi_delete_meta"})
	if le.WholeBuffer() != "" {
		t.Fatalf("dd =%q", le.WholeBuffer())
	}
}

func TestViYankOperatorAndPaste(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"y", "vi_yank"})
	feed(le, keyEv{"w", "vi_next_word"})
	// clipboard now "foo "; paste after cursor
	feed(le, keyEv{"p", "vi_paste_next"})
	if le.WholeBuffer() == "" {
		t.Fatalf("yank+paste empty")
	}
}

func TestViChangeToEol(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"f", "ed_next_char"}, keyEv{"o", "ed_next_char"})
	feed(le, keyEv{"C", "vi_change_to_eol"})
	if le.config.EditingMode() != ModeViInsert {
		t.Fatal("C should enter insert")
	}
}

func TestViReplaceChar(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"r", "vi_replace_char"})
	feed(le, keyEv{"H", "ed_insert"}) // waiting proc consumes this
	if le.WholeBuffer() != "Hello" {
		t.Fatalf("r =%q", le.WholeBuffer())
	}
}

func TestViReplaceCharCount(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"3", "ed_argument_digit"})
	feed(le, keyEv{"r", "vi_replace_char"})
	feed(le, keyEv{"x", "ed_insert"})
	if le.WholeBuffer() != "xxxlo" {
		t.Fatalf("3r =%q", le.WholeBuffer())
	}
}

func TestViFindChar(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"f", "vi_next_char"})
	feed(le, keyEv{"o", "ed_insert"})
	if le.BytePointer() != 4 {
		t.Fatalf("fo ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"t", "vi_to_next_char"})
	feed(le, keyEv{"d", "ed_insert"})
	if le.BytePointer() != 9 {
		t.Fatalf("td ptr=%d", le.BytePointer())
	}
}

func TestViFindCharBackward(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"$", "ed_move_to_end"})
	feed(le, keyEv{"F", "vi_prev_char"})
	feed(le, keyEv{"o", "ed_insert"})
	if le.BytePointer() != 7 {
		t.Fatalf("Fo ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"T", "vi_to_prev_char"})
	feed(le, keyEv{"w", "ed_insert"})
	if le.BytePointer() != 7 {
		t.Fatalf("Tw ptr=%d", le.BytePointer())
	}
}

func TestViToColumn(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"3", "ed_argument_digit"})
	feed(le, keyEv{"|", "vi_to_column"})
	if le.BytePointer() != 2 {
		t.Fatalf("3| ptr=%d", le.BytePointer())
	}
}

func TestViJoinLines(t *testing.T) {
	le := newViEditor()
	le.MultilineOn()
	feed(le, insertStr("foo")...)
	feed(le, keyEv{"\n", "key_newline"})
	feed(le, insertStr("  bar")...)
	// move to first line
	le.lineIndex = 0
	le.bytePointer = 0
	feed(le, keyEv{"J", "vi_join_lines"})
	if le.WholeBuffer() != "foo bar" {
		t.Fatalf("J =%q", le.WholeBuffer())
	}
}

func TestViInsertVariants(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"I", "vi_insert_at_bol"})
	feed(le, keyEv{"X", "ed_insert"})
	if le.WholeBuffer() != "Xhello" {
		t.Fatalf("I =%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"A", "vi_add_at_eol"})
	feed(le, keyEv{"Z", "ed_insert"})
	if le.WholeBuffer() != "XhelloZ" {
		t.Fatalf("A =%q", le.WholeBuffer())
	}
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"i", "vi_insert"})
	if le.config.EditingMode() != ModeViInsert {
		t.Fatal("i mode")
	}
}

func TestViEditingModeSwitch(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, keyEv{"x", "vi_editing_mode"})
	if le.config.EditingMode() != ModeViInsert {
		t.Fatal("vi_editing_mode")
	}
	feed(le, keyEv{"x", "emacs_editing_mode"})
	if le.config.EditingMode() != ModeEmacs {
		t.Fatal("emacs_editing_mode")
	}
}

func TestViListOrEof(t *testing.T) {
	le := newViEditor()
	feed(le, keyEv{"\x04", "vi_list_or_eof"})
	if !le.EOF() {
		t.Fatal("vi eof on empty")
	}
	le2 := newViEditor()
	feed(le2, insertStr("x")...)
	feed(le2, keyEv{"\n", "vi_list_or_eof"})
	if !le2.Finished() {
		t.Fatal("vi list_or_eof should newline-finish nonempty")
	}
}

func TestViFirstPrint(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("   hi")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"^", "vi_first_print"})
	if le.BytePointer() != 3 {
		t.Fatalf("^ ptr=%d", le.BytePointer())
	}
}

func TestEdDeletePrevCharVi(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("abc")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"X", "ed_delete_prev_char"})
	if le.WholeBuffer() != "ac" {
		t.Fatalf("X =%q", le.WholeBuffer())
	}
}

func TestViToHistoryLine(t *testing.T) {
	le := newViEditor()
	le.hist.Append("old one")
	le.hist.Append("old two")
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"G", "vi_to_history_line"})
	if le.WholeBuffer() != "old one" {
		t.Fatalf("G =%q", le.WholeBuffer())
	}
	// empty history: no-op
	le2 := newViEditor()
	feed(le2, keyEv{"G", "vi_to_history_line"})
	if le2.WholeBuffer() != "" {
		t.Fatalf("G empty =%q", le2.WholeBuffer())
	}
}

func TestViWaitingOperatorIgnoredOnNonMotion(t *testing.T) {
	le := newViEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x1b", "vi_command_mode"})
	feed(le, keyEv{"0", "vi_zero"})
	feed(le, keyEv{"d", "vi_delete_meta"})
	// follow with a non-motion that is not an operator -> cleanup, no delete
	feed(le, keyEv{"i", "vi_insert"})
	if le.WholeBuffer() != "foo bar" {
		t.Fatalf("d then i should not delete: %q", le.WholeBuffer())
	}
}
