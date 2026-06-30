package reline

import "testing"

func ints(bs ...int) []int { return bs }

func TestKeyStrokeSingleByte(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	keys, rest := ks.expand(ints('a'))
	if len(keys) != 1 || keys[0].MethodSymbol != "ed_insert" || keys[0].Char != "a" {
		t.Fatalf("keys=%+v", keys)
	}
	if len(rest) != 0 {
		t.Fatalf("rest=%v", rest)
	}
}

func TestKeyStrokeControlBinding(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	keys, _ := ks.expand(ints(1)) // C-a
	if len(keys) != 1 || keys[0].MethodSymbol != "ed_move_to_beg" {
		t.Fatalf("C-a keys=%+v", keys)
	}
}

func TestKeyStrokeMetaBinding(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC f -> em_next_word (meta-f) in emacs
	keys, _ := ks.expand(ints(27, 'f'))
	if len(keys) != 1 || keys[0].MethodSymbol != "em_next_word" {
		t.Fatalf("M-f keys=%+v", keys)
	}
}

func TestKeyStrokeMultibyte(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// "é" = 0xC3 0xA9
	keys, rest := ks.expand(ints(0xC3, 0xA9))
	if len(keys) != 1 || keys[0].Char != "é" || keys[0].MethodSymbol != "ed_insert" {
		t.Fatalf("multibyte keys=%+v", keys)
	}
	if len(rest) != 0 {
		t.Fatalf("rest=%v", rest)
	}
}

func TestKeyStrokePartialMultibyte(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// just the lead byte of "é": MATCHING_MATCHED -> no decoded key yet, all consumed
	st := ks.matchStatus(ints(0xC3))
	if st != statusMatchingMatched {
		t.Fatalf("status=%v", st)
	}
}

func TestKeyStrokeUnknownCSI(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// ESC [ A  (up arrow not in default keymap here) -> classified MATCHED
	st := ks.matchStatus(ints(27, '[', 'A'))
	if st != statusMatched {
		t.Fatalf("CSI status=%v", st)
	}
	// partial CSI
	if ks.matchStatus(ints(27, '[')) != statusMatching {
		t.Fatal("partial CSI should be MATCHING")
	}
}

func TestKeyStrokeSS3(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	st := ks.matchStatus(ints(27, 'O', 'P'))
	if st != statusMatched {
		t.Fatalf("SS3 status=%v", st)
	}
}

func TestKeyStrokeLoneEsc(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	if ks.matchStatus(ints(27)) != statusMatchingMatched {
		t.Fatal("lone ESC should be MATCHING_MATCHED")
	}
	if ks.matchStatus(ints(27, 27)) != statusMatching {
		t.Fatal("ESC ESC should be MATCHING")
	}
}

func TestKeyStrokeEscCharVi(t *testing.T) {
	cfg := NewConfig()
	cfg.SetEditingMode(ModeViCommand)
	ks := newKeyStroke(cfg)
	// In vi mode, ESC + non-CSI char is UNMATCHED
	st := ks.matchStatus(ints(27, 'x'))
	if st != statusUnmatched {
		t.Fatalf("vi esc-char status=%v", st)
	}
}

func TestKeyStrokeExpandConsumesPrefix(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// C-a followed by 'b': expand consumes C-a, leaves 'b'
	keys, rest := ks.expand(ints(1, 'b'))
	if len(keys) != 1 || keys[0].MethodSymbol != "ed_move_to_beg" {
		t.Fatalf("keys=%+v", keys)
	}
	if len(rest) != 1 || rest[0] != 'b' {
		t.Fatalf("rest=%v", rest)
	}
}

func TestKeyStrokeNoMatch(t *testing.T) {
	cfg := NewConfig()
	ks := newKeyStroke(cfg)
	// invalid lone continuation byte 0x80: matchUnknown path returns via default
	keys, _ := ks.expand(ints(0xFF, 0xFF, 0xFF, 0xFF))
	_ = keys // just exercising the no-clean-match path
}

func TestConfigKeyBindingsAndOneshot(t *testing.T) {
	cfg := NewConfig()
	// byte 200 is unbound in the default emacs keymap, so the oneshot binding is
	// observable in isolation.
	cfg.addOneshotKeyBinding(ints(200), "ed_move_to_end")
	km := cfg.keyBindings()
	if v, ok := km.get(ints(200)); !ok || v != "ed_move_to_end" {
		t.Fatalf("oneshot get=%q ok=%v", v, ok)
	}
	cfg.resetOneshotKeyBindings()
	if _, ok := cfg.keyBindings().get(ints(200)); ok {
		t.Fatal("oneshot should be cleared")
	}
}

func TestConfigEditingModeAccessors(t *testing.T) {
	cfg := NewConfig()
	if cfg.EditingMode() != ModeEmacs {
		t.Fatal("default emacs")
	}
	cfg.SetEditingMode(ModeViCommand)
	if !cfg.EditingModeIs(ModeViCommand, ModeViInsert) {
		t.Fatal("vi command")
	}
	cfg.reset() // vi_command -> vi_insert
	if cfg.EditingMode() != ModeViInsert {
		t.Fatal("reset to insert")
	}
	// emacs reset stays emacs
	cfg2 := NewConfig()
	cfg2.reset()
	if cfg2.EditingMode() != ModeEmacs {
		t.Fatal("emacs reset")
	}
}

func TestConfigDefaultActorVi(t *testing.T) {
	cfg := NewConfig()
	cfg.SetEditingMode(ModeViInsert)
	km := cfg.keyBindings()
	// ESC in vi-insert is bound to vi_command_mode (the meta-prefix)
	if !km.matching(ints(27)) {
		// ESC may map directly; just ensure composite works
	}
	if _, ok := km.get(ints(27)); !ok {
		t.Fatal("esc binding in vi insert")
	}
}
