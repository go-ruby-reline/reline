package reline

// ReadKeys reads the next key from io and decodes it with the keymap of
// config's current mode (MRI Reline::Core#read_io): bytes are read until
// they match a binding, or match nothing. A sequence that is a binding and the
// prefix of a longer one (ESC alone, or ESC that starts a CSI sequence) waits
// config.KeyseqTimeout milliseconds for the next byte before it is taken as
// it is. Bytes read past the decoded key are pushed back with UngetC.
//
// It returns the decoded keys: usually one, none for a sequence that is
// bound to nothing (an unknown escape sequence, an invalid byte), and a
// single EOF key when io reports the end of its input before any byte.
func ReadKeys(config *Config, io IO) []Key {
	ks := newKeyStroke(config)
	var buffer []int
	status := statusMatching
	for {
		timeout := -1
		if status == statusMatchingMatched {
			timeout = config.KeyseqTimeout
		}
		c := io.GetC(timeout)
		if c < 0 {
			switch {
			case status == statusMatchingMatched:
				status = statusMatched
			case len(buffer) == 0:
				return []Key{{EOF: true}}
			default:
				status = statusUnmatched
			}
		} else {
			buffer = append(buffer, c)
			status = ks.matchStatus(buffer)
		}
		if status == statusMatched || status == statusUnmatched {
			expanded, rest := ks.expand(buffer)
			for i := len(rest) - 1; i >= 0; i-- {
				io.UngetC(rest[i])
			}
			return expanded
		}
	}
}
