package reline

// EditingMode selects the active keymap (MRI @editing_mode_label).
type EditingMode int

const (
	// ModeEmacs is the default emacs keymap.
	ModeEmacs EditingMode = iota
	// ModeViInsert is vi insert mode.
	ModeViInsert
	// ModeViCommand is vi command (normal) mode.
	ModeViCommand
)

// Config holds the editor configuration relevant to the pure editing core
// (MRI Reline::Config, trimmed to the state-machine-relevant fields).
type Config struct {
	editingMode          EditingMode
	HistorySize          int  // -1 unlimited (default), 0 drop-all, >0 cap
	CompletionIgnoreCase bool // case-fold completion matching
	ShowAllIfAmbiguous   bool // show menu immediately on ambiguous completion
	DisableCompletion    bool
	Autocompletion       bool
	KeyseqTimeout        int // ms; ESC ambiguity timeout
	IsearchTerminators   string

	emacsActor     *keyActor
	viInsertActor  *keyActor
	viCommandActor *keyActor
	oneshot        *keyActor
	additional     map[EditingMode]*keyActor
}

// NewConfig returns a Config with MRI's defaults (emacs mode, unlimited history,
// 500ms keyseq timeout) and the standard emacs/vi keymaps loaded.
func NewConfig() *Config {
	c := &Config{
		editingMode:   ModeEmacs,
		HistorySize:   -1,
		KeyseqTimeout: 500,
		additional:    map[EditingMode]*keyActor{},
	}
	c.emacsActor = newKeyActorFromMapping(emacsMapping)
	c.viInsertActor = newKeyActorFromMapping(viInsertMapping)
	c.viCommandActor = newKeyActorFromMapping(viCommandMapping)
	c.oneshot = newEmptyKeyActor()
	c.additional[ModeEmacs] = newEmptyKeyActor()
	c.additional[ModeViInsert] = newEmptyKeyActor()
	c.additional[ModeViCommand] = newEmptyKeyActor()
	return c
}

// EditingMode returns the current mode.
func (c *Config) EditingMode() EditingMode { return c.editingMode }

// SetEditingMode switches the active keymap.
func (c *Config) SetEditingMode(m EditingMode) { c.editingMode = m }

// EditingModeIs reports whether the current mode is any of the given modes.
func (c *Config) EditingModeIs(modes ...EditingMode) bool {
	for _, m := range modes {
		if c.editingMode == m {
			return true
		}
	}
	return false
}

func (c *Config) defaultActor(m EditingMode) *keyActor {
	switch m {
	case ModeViInsert:
		return c.viInsertActor
	case ModeViCommand:
		return c.viCommandActor
	default:
		return c.emacsActor
	}
}

// keyBindings returns the layered lookup for the current mode (MRI
// Config#key_bindings): oneshot, then inputrc additions, then the default map.
func (c *Config) keyBindings() *compositeKeyActor {
	return &compositeKeyActor{actors: []*keyActor{
		c.oneshot,
		c.additional[c.editingMode],
		c.defaultActor(c.editingMode),
	}}
}

// addOneshotKeyBinding registers a one-shot binding consumed on the next key
// (used by dialog trap keys); mirrors Config#add_oneshot_key_binding.
func (c *Config) addOneshotKeyBinding(keystroke []int, fn string) {
	c.oneshot.add(keystroke, fn)
}

// resetOneshotKeyBindings clears one-shot bindings (called each key cycle).
func (c *Config) resetOneshotKeyBindings() { c.oneshot.clear() }

// reset restores the editing mode to its insert variant on finish
// (MRI Config#reset: vi_command -> vi_insert).
func (c *Config) reset() {
	if c.EditingModeIs(ModeViCommand) {
		c.editingMode = ModeViInsert
	}
	c.oneshot.clear()
}
