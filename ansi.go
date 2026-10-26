package reline

// ansiCursorKeys are the final bytes of the ANSI cursor keys with their
// command and, for Right and Left, the command with Ctrl or Meta held (MRI
// Reline::ANSI::ANSI_CURSOR_KEY_BINDINGS).
var ansiCursorKeys = []struct {
	char       byte
	fn, ctrlFn string
}{
	{'A', "ed_prev_history", ""},          // Up
	{'B', "ed_next_history", ""},          // Down
	{'C', "ed_next_char", "em_next_word"}, // Right
	{'D', "ed_prev_char", "ed_prev_word"}, // Left
	{'F', "ed_move_to_end", ""},           // End
	{'H', "ed_move_to_beg", ""},           // Home
}

// StartBracketedPaste and EndBracketedPaste delimit text a terminal in
// bracketed-paste mode pastes (MRI Reline::ANSI::START_BRACKETED_PASTE /
// END_BRACKETED_PASTE). SetANSIKeyBindings binds the start to the command
// "bracketed_paste_start", which a host answers by reading up to the end and
// inserting the text (LineEditor.InsertMultilineText).
const (
	StartBracketedPaste = "\x1b[200~"
	EndBracketedPaste   = "\x1b[201~"
)

var allModes = []EditingMode{ModeEmacs, ModeViInsert, ModeViCommand}

// SetANSIKeyBindings adds the default bindings of an ANSI (VT100/xterm)
// terminal to c (MRI Reline::ANSI#set_default_key_bindings): the cursor keys
// in their CSI and SS3 forms, with Ctrl and Meta for Right and Left; Delete,
// Page Up/Down, and the console and urxvt Home/End; Shift-Tab; bracketed
// paste; and, in emacs mode, M-space and C-x C-x. A host whose terminal sends
// these sequences (any VT terminal, and the Windows console with virtual
// terminal input) calls it once on its Config.
func SetANSIKeyBindings(c *Config) {
	for _, m := range allModes {
		c.AddDefaultKeyBinding(m, []byte(StartBracketedPaste), "bracketed_paste_start")
	}
	for _, k := range ansiCursorKeys {
		bindings := [][2]string{
			{"\x1b[" + string(k.char), k.fn}, // CSI + char
			{"\x1bO" + string(k.char), k.fn}, // SS3 + char, application cursor key mode
		}
		if k.ctrlFn != "" {
			// In MRI, Ctrl and Meta give the same command for both keys.
			bindings = append(bindings,
				[2]string{"\x1b[1;5" + string(k.char), k.ctrlFn},  // CSI + ctrl modifier + char
				[2]string{"\x1b[1;3" + string(k.char), k.ctrlFn},  // CSI + meta modifier + char
				[2]string{"\x1b\x1b[" + string(k.char), k.ctrlFn}, // Meta (ESC) + CSI + char
			)
		}
		for _, b := range bindings {
			for _, m := range allModes {
				c.AddDefaultKeyBinding(m, []byte(b[0]), b[1])
			}
		}
	}
	for _, b := range [][2]string{
		{"\x1b[3~", "key_delete"},             // xterm kdch1
		{"\x1b[5~", "ed_search_prev_history"}, // kpp
		{"\x1b[6~", "ed_search_next_history"}, // knp
		{"\x1b[1~", "ed_move_to_beg"},         // console Home
		{"\x1b[4~", "ed_move_to_end"},         // console End
		{"\x1b[7~", "ed_move_to_beg"},         // urxvt Home
		{"\x1b[8~", "ed_move_to_end"},         // urxvt End
	} {
		for _, m := range allModes {
			c.AddDefaultKeyBinding(m, []byte(b[0]), b[1])
		}
	}
	c.AddDefaultKeyBinding(ModeEmacs, []byte("\x1b[Z"), "completion_journey_up") // S-Tab
	c.AddDefaultKeyBinding(ModeViInsert, []byte("\x1b[Z"), "completion_journey_up")
	c.AddDefaultKeyBinding(ModeEmacs, []byte("\x1b "), "em_set_mark")         // M-<space>
	c.AddDefaultKeyBinding(ModeEmacs, []byte("\x18\x18"), "em_exchange_mark") // C-x C-x
}
