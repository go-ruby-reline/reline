package reline

import (
	"strings"
	"testing"
)

// keyEv is a scripted key event: the input string and the command symbol it
// dispatches to (as the keymap would resolve it).
type keyEv struct {
	s   string
	sym string
}

func newTestEditor() (*LineEditor, *Config, *NullIO) {
	cfg := NewConfig()
	io := &NullIO{Rows: 24, Cols: 80}
	hist := NewHistory(-1)
	le := NewLineEditor(cfg, io, hist)
	le.Reset("> ")
	return le, cfg, io
}

func feed(le *LineEditor, evs ...keyEv) {
	for _, e := range evs {
		le.Update(Key{Char: e.s, MethodSymbol: e.sym})
	}
}

// insertStr feeds each rune of s as an ed_insert key.
func insertStr(s string) []keyEv {
	var out []keyEv
	for _, r := range s {
		out = append(out, keyEv{string(r), "ed_insert"})
	}
	return out
}

func lineOf(le *LineEditor) string {
	l, _ := le.Line()
	return l
}

func TestInsertAndCursor(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello")...)
	if got := lineOf(le); got != "hello" {
		t.Fatalf("line=%q", got)
	}
	if le.BytePointer() != 5 {
		t.Fatalf("ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"\x01", "ed_move_to_beg"}, keyEv{"X", "ed_insert"})
	if got := lineOf(le); got != "Xhello" {
		t.Fatalf("line=%q", got)
	}
	if le.BytePointer() != 1 {
		t.Fatalf("ptr=%d", le.BytePointer())
	}
}

func TestMovementCommands(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	if le.BytePointer() != 0 {
		t.Fatal("beg")
	}
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	if le.BytePointer() != 7 {
		t.Fatal("end")
	}
	feed(le, keyEv{"\x02", "ed_prev_char"})
	if le.BytePointer() != 6 {
		t.Fatal("prev")
	}
	feed(le, keyEv{"\x06", "ed_next_char"})
	if le.BytePointer() != 7 {
		t.Fatal("next")
	}
	// word moves
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"f", "em_next_word"})
	if le.BytePointer() != 3 {
		t.Fatalf("forward word ptr=%d", le.BytePointer())
	}
	feed(le, keyEv{"b", "ed_prev_word"})
	if le.BytePointer() != 0 {
		t.Fatalf("backward word ptr=%d", le.BytePointer())
	}
}

func TestKillLineAndYank(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("foobar")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"}, keyEv{"\x0b", "ed_kill_line"})
	if lineOf(le) != "" {
		t.Fatalf("after kill: %q", lineOf(le))
	}
	feed(le, keyEv{"\x19", "em_yank"})
	if lineOf(le) != "foobar" {
		t.Fatalf("after yank: %q", lineOf(le))
	}
}

func TestKillWordAndKillRingAccumulate(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("alpha beta gamma")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	// two consecutive forward kill-words accumulate into one ring entry
	feed(le, keyEv{"d", "em_delete_next_word"})
	feed(le, keyEv{"d", "em_delete_next_word"})
	if lineOf(le) != " gamma" {
		t.Fatalf("line=%q", lineOf(le))
	}
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	feed(le, keyEv{"\x19", "em_yank"})
	if lineOf(le) != " gammaalpha beta" {
		t.Fatalf("yank accumulate=%q", lineOf(le))
	}
}

func TestBackwardKillWordPrepends(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("alpha beta")...)
	feed(le, keyEv{"\x17", "ed_delete_prev_word"})
	feed(le, keyEv{"\x17", "ed_delete_prev_word"})
	if lineOf(le) != "" {
		t.Fatalf("line=%q", lineOf(le))
	}
	feed(le, keyEv{"\x19", "em_yank"})
	if lineOf(le) != "alpha beta" {
		t.Fatalf("yank=%q", lineOf(le))
	}
}

func TestTransposeChars(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("ab")...)
	feed(le, keyEv{"\x14", "ed_transpose_chars"})
	if lineOf(le) != "ba" {
		t.Fatalf("line=%q", lineOf(le))
	}
}

func TestTransposeWords(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("foo bar")...)
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	feed(le, keyEv{"t", "ed_transpose_words"})
	if lineOf(le) != "bar foo" {
		t.Fatalf("line=%q", lineOf(le))
	}
}

func TestCaseWords(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("foo bar baz")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"u", "em_upper_case"})
	if lineOf(le) != "FOO bar baz" {
		t.Fatalf("upper=%q", lineOf(le))
	}
	feed(le, keyEv{"c", "em_capitol_case"})
	if lineOf(le) != "FOO Bar baz" {
		t.Fatalf("capitol=%q", lineOf(le))
	}
	feed(le, keyEv{"l", "em_lower_case"})
	if lineOf(le) != "FOO Bar baz" {
		// cursor now after "Bar"; lower-case next word "baz" -> unchanged (already lower)
		t.Fatalf("lower=%q", lineOf(le))
	}
}

func TestDeleteChar(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"\x04", "em_delete"})
	if lineOf(le) != "ello" {
		t.Fatalf("line=%q", lineOf(le))
	}
}

func TestBackspace(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x08", "em_delete_prev_char"})
	if lineOf(le) != "hell" {
		t.Fatalf("line=%q", lineOf(le))
	}
}

func TestUnixLineDiscardAndWholeLine(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello world")...)
	feed(le, keyEv{"\x15", "unix_line_discard"}) // kill to bol (whole, ptr at end)
	if lineOf(le) != "" {
		t.Fatalf("discard=%q", lineOf(le))
	}
	feed(le, keyEv{"\x19", "em_yank"})
	if lineOf(le) != "hello world" {
		t.Fatalf("yank=%q", lineOf(le))
	}
	// kill_whole_line
	le2, _, _ := newTestEditor()
	feed(le2, insertStr("xyz")...)
	feed(le2, keyEv{"k", "em_kill_line"})
	if lineOf(le2) != "" {
		t.Fatalf("kill whole=%q", lineOf(le2))
	}
}

func TestYankPop(t *testing.T) {
	le, _, _ := newTestEditor()
	// build two kill ring entries
	feed(le, insertStr("first")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"\x0b", "ed_kill_line"}) // kill "first"
	feed(le, insertStr("second")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"\x0b", "ed_kill_line"}) // kill "second"
	feed(le, keyEv{"\x19", "em_yank"})      // yank "second"
	if lineOf(le) != "second" {
		t.Fatalf("yank=%q", lineOf(le))
	}
	feed(le, keyEv{"y", "em_yank_pop"}) // rotate to "first"
	if lineOf(le) != "first" {
		t.Fatalf("yank_pop=%q", lineOf(le))
	}
}

func TestNewlineFinishesSingleLine(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("done")...)
	feed(le, keyEv{"\n", "ed_newline"})
	if !le.Finished() {
		t.Fatal("not finished")
	}
	if lineOf(le) != "done" {
		t.Fatalf("line=%q", lineOf(le))
	}
}

func TestEOFEmpty(t *testing.T) {
	le, _, _ := newTestEditor()
	le.Update(Key{EOF: true})
	if !le.EOF() || !le.Finished() {
		t.Fatal("expected eof+finished")
	}
	if _, ok := le.Line(); ok {
		t.Fatal("expected nil line at eof")
	}
}

func TestCtrlDOnEmptyIsEOF(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, keyEv{"\x04", "em_delete"})
	if !le.EOF() {
		t.Fatal("ctrl-d empty should be eof")
	}
}

func TestUndoRedo(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("abc")...)
	feed(le, insertStr("def")...)
	if lineOf(le) != "abcdef" {
		t.Fatalf("line=%q", lineOf(le))
	}
	feed(le, keyEv{"\x1f", "undo"})
	first := lineOf(le)
	feed(le, keyEv{"\x12", "redo"})
	if lineOf(le) == first && first == "abcdef" {
		t.Fatalf("redo did not change after undo: %q", lineOf(le))
	}
}

func TestMultilineNewline(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("line1")...)
	feed(le, keyEv{"\n", "ed_newline"})
	feed(le, insertStr("line2")...)
	if le.WholeBuffer() != "line1\nline2" {
		t.Fatalf("buffer=%q", le.WholeBuffer())
	}
	if le.LineIndex() != 1 {
		t.Fatalf("line index=%d", le.LineIndex())
	}
	// With a termination predicate that accepts, Enter on the last line submits.
	le.SetConfirmMultilineTermination(func(string) bool { return true })
	feed(le, keyEv{"\n", "ed_newline"})
	if !le.Finished() {
		t.Fatal("multiline did not finish on last line")
	}
}

func TestMultilineTerminationFalseKeepsEditing(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	le.SetConfirmMultilineTermination(func(string) bool { return false })
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "ed_newline"})
	feed(le, keyEv{"\n", "ed_newline"})
	if le.Finished() {
		t.Fatal("should not finish when predicate is false")
	}
	if len(le.WholeLines()) != 3 {
		t.Fatalf("lines=%d", len(le.WholeLines()))
	}
}

func TestInsertMultilineText(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	le.InsertMultilineText("a\r\nb\rc")
	if le.WholeBuffer() != "a\nb\nc" {
		t.Fatalf("buffer=%q", le.WholeBuffer())
	}
}

func TestSetMarkExchange(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello")...)
	feed(le, keyEv{"\x01", "ed_move_to_beg"})
	feed(le, keyEv{"\x00", "em_set_mark"})
	feed(le, keyEv{"\x05", "ed_move_to_end"})
	feed(le, keyEv{"x", "em_exchange_mark"})
	if le.BytePointer() != 0 {
		t.Fatalf("exchange ptr=%d", le.BytePointer())
	}
}

func TestMultibyteInsertDelete(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, keyEv{"é", "ed_insert"}, keyEv{"漢", "ed_insert"}, keyEv{"字", "ed_insert"})
	if lineOf(le) != "é漢字" {
		t.Fatalf("line=%q", lineOf(le))
	}
	// width: é=1, 漢=2, 字=2 => 5
	if w := CalculateWidth(lineOf(le), false); w != 5 {
		t.Fatalf("width=%d", w)
	}
	feed(le, keyEv{"\x08", "em_delete_prev_char"})
	if lineOf(le) != "é漢" {
		t.Fatalf("after bs=%q", lineOf(le))
	}
}

func TestSubmittedNewlineEmpty(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, keyEv{"\n", "ed_newline"})
	if !le.Finished() || lineOf(le) != "" {
		t.Fatalf("finished=%v line=%q", le.Finished(), lineOf(le))
	}
}

func TestPasteState(t *testing.T) {
	le, _, _ := newTestEditor()
	le.SetPastingState(true)
	feed(le, keyEv{"a", "ed_insert"}, keyEv{"b", "ed_insert"})
	// nothing inserted yet (buffered)
	if lineOf(le) != "" {
		t.Fatalf("during paste=%q", lineOf(le))
	}
	le.SetPastingState(false)
	if lineOf(le) != "ab" {
		t.Fatalf("after paste=%q", lineOf(le))
	}
}

func TestRenderBasic(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr("hello")...)
	rs := le.Render(80, nil)
	if len(rs.Lines) != 1 {
		t.Fatalf("lines=%d", len(rs.Lines))
	}
	if rs.Lines[0][0] != "> " || rs.Lines[0][1] != "hello" {
		t.Fatalf("render=%q/%q", rs.Lines[0][0], rs.Lines[0][1])
	}
	// cursor after "hello" => x = 2 (prompt) + 5 = 7
	if rs.CursorX != 7 || rs.CursorY != 0 {
		t.Fatalf("cursor=%d,%d", rs.CursorX, rs.CursorY)
	}
}

func TestRenderWrap(t *testing.T) {
	le, _, _ := newTestEditor()
	feed(le, insertStr(strings.Repeat("a", 10))...)
	rs := le.Render(6, nil)
	if len(rs.Lines) < 2 {
		t.Fatalf("expected wrap, got %d lines", len(rs.Lines))
	}
}

func TestRenderMultilinePrompt(t *testing.T) {
	le, _, _ := newTestEditor()
	le.MultilineOn()
	feed(le, insertStr("a")...)
	feed(le, keyEv{"\n", "ed_newline"})
	feed(le, insertStr("b")...)
	pp := func(lines []string) []string {
		out := make([]string, len(lines))
		for i := range lines {
			out[i] = "ruby> "
		}
		return out
	}
	rs := le.Render(80, pp)
	if len(rs.Lines) != 2 {
		t.Fatalf("lines=%d", len(rs.Lines))
	}
	if rs.Lines[0][0] != "ruby> " {
		t.Fatalf("prompt=%q", rs.Lines[0][0])
	}
}
