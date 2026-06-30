package reline

import "unicode/utf8"

// matchStatus values mirror Reline::KeyStroke's MATCHING / MATCHED /
// MATCHING_MATCHED / UNMATCHED.
type matchStatus int

const (
	statusMatching        matchStatus = iota // partial prefix of a binding
	statusMatched                            // exactly a binding (or one char)
	statusMatchingMatched                    // a binding and a prefix of another
	statusUnmatched                          // matches nothing
)

const escByte = 27

// Key is a decoded key event: Char is the input string and MethodSymbol the
// command it dispatches to (MRI Reline::Key). A nil/empty Char with empty
// MethodSymbol represents EOF.
type Key struct {
	Char         string
	MethodSymbol string
	EOF          bool
}

// keyStroke decodes raw input bytes into Keys using the config's layered
// keymap (MRI Reline::KeyStroke).
type keyStroke struct {
	config *Config
}

func newKeyStroke(config *Config) *keyStroke {
	return &keyStroke{config: config}
}

func (ks *keyStroke) matchStatus(input []int) matchStatus {
	km := ks.config.keyBindings()
	matching := km.matching(input)
	_, matched := km.get(input)
	switch {
	case matching && matched:
		return statusMatchingMatched
	case matching:
		return statusMatching
	case matched:
		return statusMatched
	case len(input) > 0 && input[0] == escByte:
		return ks.matchUnknownEscapeSequence(input, ks.config.EditingModeIs(ModeViInsert, ModeViCommand))
	default:
		b := bytesOf(input)
		if utf8.Valid(b) {
			if utf8.RuneCount(b) == 1 {
				return statusMatched
			}
			return statusUnmatched
		}
		// Invalid byte sequence: still matching (a prefix) or matched (bytes
		// to be ignored).
		return statusMatchingMatched
	}
}

func bytesOf(input []int) []byte {
	b := make([]byte, len(input))
	for i, v := range input {
		b[i] = byte(v)
	}
	return b
}

// expand consumes the longest matching prefix of input and returns the decoded
// Keys plus the remaining (unconsumed) bytes (MRI KeyStroke#expand).
func (ks *keyStroke) expand(input []int) ([]Key, []int) {
	var matchedBytes []int
	for i := 1; i <= len(input); i++ {
		bytes := input[:i]
		status := ks.matchStatus(bytes)
		if status == statusMatched || status == statusMatchingMatched {
			matchedBytes = append([]int(nil), bytes...)
		}
		if status == statusMatched || status == statusUnmatched {
			break
		}
	}
	if matchedBytes == nil {
		return nil, nil
	}

	km := ks.config.keyBindings()
	fn, _ := km.get(matchedBytes)
	s := string(bytesOf(matchedBytes))
	var keys []Key
	if fn != "" {
		keys = []Key{{Char: s, MethodSymbol: fn}}
	} else {
		if utf8.ValidString(s) && utf8.RuneCountInString(s) == 1 {
			keys = []Key{{Char: s, MethodSymbol: "ed_insert"}}
		}
	}
	return keys, input[len(matchedBytes):]
}

// matchUnknownEscapeSequence classifies an ESC-prefixed CSI/SS3 sequence not in
// the keymap (MRI KeyStroke#match_unknown_escape_sequence).
func (ks *keyStroke) matchUnknownEscapeSequence(input []int, viMode bool) matchStatus {
	at := func(i int) (int, bool) {
		if i >= 0 && i < len(input) {
			return input[i], true
		}
		return 0, false
	}
	idx := 0
	if v, _ := at(idx); v != escByte {
		return statusUnmatched
	}
	idx++
	if v, ok := at(idx); ok && v == escByte {
		idx++
	}
	v, ok := at(idx)
	switch {
	case !ok:
		if idx == 1 { // ESC
			return statusMatchingMatched
		}
		return statusMatching // ESC ESC
	case v == 91: // '['
		idx++
		for {
			c, ok := at(idx)
			if !ok || !(c >= 0x30 && c <= 0x3f) {
				break
			}
			idx++
		}
		for {
			c, ok := at(idx)
			if !ok || !(c >= 0x20 && c <= 0x2f) {
				break
			}
			idx++
		}
	case v == 79: // 'O' SS3
		idx++
	default: // ESC char / ESC ESC char
		if viMode {
			return statusUnmatched
		}
	}
	switch len(input) {
	case idx:
		return statusMatching
	case idx + 1:
		return statusMatched
	default:
		return statusUnmatched
	}
}
