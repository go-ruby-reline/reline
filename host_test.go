package reline

import (
	"reflect"
	"testing"
)

// script is a NullIO whose input is s: NullIO pops its pushed-back bytes
// last first, so they are pushed in reverse.
func script(s string) *NullIO {
	io := &NullIO{}
	for i := len(s) - 1; i >= 0; i-- {
		io.UngetC(int(s[i]))
	}
	return io
}

func ansiConfig() *Config {
	c := NewConfig()
	SetANSIKeyBindings(c)
	return c
}

func TestReadKeysANSICursorKeys(t *testing.T) {
	c := ansiConfig()
	for seq, want := range map[string]string{
		"\x1b[A": "ed_prev_history", "\x1bOB": "ed_next_history",
		"\x1b[C": "ed_next_char", "\x1b[D": "ed_prev_char",
		"\x1b[1;5C": "em_next_word", "\x1b[1;3D": "ed_prev_word", "\x1b\x1b[C": "em_next_word",
		"\x1b[H": "ed_move_to_beg", "\x1b[F": "ed_move_to_end",
		"\x1b[3~": "key_delete", "\x1b[5~": "ed_search_prev_history", "\x1b[6~": "ed_search_next_history",
		"\x1b[1~": "ed_move_to_beg", "\x1b[4~": "ed_move_to_end", "\x1b[7~": "ed_move_to_beg", "\x1b[8~": "ed_move_to_end",
		"\x1b[Z": "completion_journey_up", "\x1b ": "em_set_mark", "\x18\x18": "em_exchange_mark",
		StartBracketedPaste: "bracketed_paste_start",
	} {
		io := script(seq)
		got := ReadKeys(c, io)
		if len(got) != 1 || got[0].Char != seq || got[0].MethodSymbol != want {
			t.Errorf("%q: got %+v, want %s", seq, got, want)
		}
		if len(io.Unget) != 0 {
			t.Errorf("%q: %d bytes left", seq, len(io.Unget))
		}
	}
	// vi command mode has the cursor keys, not the emacs-only ones.
	c.SetEditingMode(ModeViCommand)
	if got := ReadKeys(c, script("\x1b[A")); got[0].MethodSymbol != "ed_prev_history" {
		t.Errorf("vi command Up: %+v", got)
	}
}

func TestReadKeysWithoutANSIBindings(t *testing.T) {
	// An escape sequence bound to nothing is read whole and gives no key.
	if got := ReadKeys(NewConfig(), script("\x1b[A")); len(got) != 0 {
		t.Errorf("unbound CSI: %+v", got)
	}
}

func TestReadKeysPlainAndRest(t *testing.T) {
	c := NewConfig()
	io := script("ab")
	if got := ReadKeys(c, io); !reflect.DeepEqual(got, []Key{{Char: "a", MethodSymbol: "ed_insert"}}) {
		t.Errorf("a: %+v", got)
	}
	if got := ReadKeys(c, io); got[0].Char != "b" {
		t.Errorf("b: %+v", got)
	}
	if got := ReadKeys(c, io); !reflect.DeepEqual(got, []Key{{EOF: true}}) {
		t.Errorf("end: %+v", got)
	}
}

func TestReadKeysLoneEscapeTimesOut(t *testing.T) {
	// ESC is a binding and a prefix: no byte within the timeout makes it ESC.
	got := ReadKeys(NewConfig(), script("\x1b"))
	if len(got) != 1 || got[0].Char != "\x1b" {
		t.Errorf("ESC: %+v", got)
	}
}

func TestReadKeysTruncatedSequencePushesBack(t *testing.T) {
	// ESC [ then the end: ESC is decoded, '[' is pushed back for the next read.
	io := script("\x1b[")
	got := ReadKeys(NewConfig(), io)
	if len(got) != 1 || got[0].Char != "\x1b" {
		t.Errorf("ESC: %+v", got)
	}
	if !reflect.DeepEqual(io.Unget, []int{'['}) {
		t.Errorf("pushed back %v", io.Unget)
	}
	if got := ReadKeys(NewConfig(), io); got[0].Char != "[" {
		t.Errorf("then: %+v", got)
	}
}

func TestReadKeysMultibyte(t *testing.T) {
	got := ReadKeys(NewConfig(), script("é"))
	if len(got) != 1 || got[0].Char != "é" || got[0].MethodSymbol != "ed_insert" {
		t.Errorf("é: %+v", got)
	}
}

func TestAddDefaultKeyBinding(t *testing.T) {
	c := NewConfig()
	c.AddDefaultKeyBinding(ModeEmacs, nil, "ed_insert") // ignored
	c.AddDefaultKeyBinding(ModeEmacs, []byte("\x1bxy"), "ed_move_to_end")
	if got := ReadKeys(c, script("\x1bxy")); len(got) != 1 || got[0].MethodSymbol != "ed_move_to_end" {
		t.Errorf("custom: %+v", got)
	}
	// It replaces the base keymap's binding of the same sequence, as in MRI.
	c.AddDefaultKeyBinding(ModeEmacs, []byte{1}, "ed_move_to_end")
	if got := ReadKeys(c, script("\x01")); got[0].MethodSymbol != "ed_move_to_end" {
		t.Errorf("replaced: %+v", got)
	}
}

func TestKeyDelete(t *testing.T) {
	for _, tc := range []struct {
		mode EditingMode
		want string
	}{{ModeEmacs, "ac"}, {ModeViInsert, "ac"}, {ModeViCommand, "abc"}} {
		le, c, _ := newTestEditor()
		feed(le, insertStr("abc")...)
		le.SetBytePointer(1)
		c.SetEditingMode(tc.mode)
		le.Update(Key{Char: "\x1b[3~", MethodSymbol: "key_delete"})
		if got := lineOf(le); got != tc.want {
			t.Errorf("mode %d: %q, want %q", tc.mode, got, tc.want)
		}
	}
	// Delete on an empty buffer is not Ctrl-D: no end of input.
	le, _, _ := newTestEditor()
	le.Update(Key{Char: "\x1b[3~", MethodSymbol: "key_delete"})
	if le.Finished() {
		t.Error("Delete on an empty line ended the read")
	}
}

func TestMenuLines(t *testing.T) {
	if got := MenuLines(nil, 80); got != nil {
		t.Errorf("empty: %q", got)
	}
	// Six names of width 5 take 7 columns each: 20 columns fit two, filled
	// column by column.
	list := []string{"ffff1", "aaaa1", "cccc1", "bbbb1", "eeee1", "dddd1"}
	want := []string{"aaaa1  dddd1", "bbbb1  eeee1", "cccc1  ffff1"}
	if got := MenuLines(list, 20); !reflect.DeepEqual(got, want) {
		t.Errorf("columns: %q", got)
	}
	if list[0] != "ffff1" {
		t.Error("MenuLines sorted its argument")
	}
	// Narrower than one item: one column.
	if got := MenuLines([]string{"b", "a"}, 1); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("one column: %q", got)
	}
	// An incomplete last column.
	if got := MenuLines([]string{"a", "b", "c"}, 6); !reflect.DeepEqual(got, []string{"a  c", "b"}) {
		t.Errorf("ragged: %q", got)
	}
}

func TestTakeMenu(t *testing.T) {
	le, _, _ := newTestEditor()
	le.SetCompletionProc(func(target, pre, post string) []string { return []string{"foo", "food"} })
	feed(le, insertStr("fo")...)
	le.Update(Key{Char: "\t", MethodSymbol: "complete"}) // "foo", a perfect match among two
	le.Update(Key{Char: "\t", MethodSymbol: "complete"}) // the menu
	list, ok := le.TakeMenu()
	if !ok || !reflect.DeepEqual(list, []string{"foo", "food"}) {
		t.Fatalf("menu %q %v", list, ok)
	}
	if _, ok := le.TakeMenu(); ok {
		t.Error("the menu is still pending after TakeMenu")
	}
	if _, ok := le.MenuInfo(); ok {
		t.Error("MenuInfo still has it")
	}
}
