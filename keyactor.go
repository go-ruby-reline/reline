package reline

// keyActor is the byte-sequence -> command lookup table (MRI
// Reline::KeyActor::Base). It is built from a 256-entry keymap: indices 0..127
// bind single bytes, and any non-nil meta entry (index|0x80) binds the
// ESC-prefixed two-byte sequence.
type keyActor struct {
	matchingBytes map[string]bool   // prefixes that may extend to a binding
	keyBindings   map[string]string // full byte sequence -> command symbol
}

func keyOf(bytes []int) string {
	b := make([]byte, len(bytes))
	for i, v := range bytes {
		b[i] = byte(v)
	}
	return string(b)
}

func newKeyActorFromMapping(mapping [256]string) *keyActor {
	ka := &keyActor{matchingBytes: map[string]bool{}, keyBindings: map[string]string{}}
	ka.add([]int{27}, "ed_ignore")
	for key := 0; key < 128; key++ {
		fn := mapping[key]
		metaFn := mapping[key|0x80]
		if fn != "" {
			ka.add([]int{key}, fn)
		}
		if metaFn != "" {
			ka.add([]int{27, key}, metaFn)
		}
	}
	return ka
}

func newEmptyKeyActor() *keyActor {
	return &keyActor{matchingBytes: map[string]bool{}, keyBindings: map[string]string{}}
}

// add registers a byte sequence -> command binding, marking all proper prefixes
// as "matching" so partial input is recognized as still-matching.
func (ka *keyActor) add(key []int, fn string) {
	for size := 1; size < len(key); size++ {
		ka.matchingBytes[keyOf(key[:size])] = true
	}
	ka.keyBindings[keyOf(key)] = fn
}

func (ka *keyActor) matching(key []int) bool { return ka.matchingBytes[keyOf(key)] }

func (ka *keyActor) get(key []int) (string, bool) {
	v, ok := ka.keyBindings[keyOf(key)]
	return v, ok
}

func (ka *keyActor) clear() {
	ka.matchingBytes = map[string]bool{}
	ka.keyBindings = map[string]string{}
}

// compositeKeyActor layers several keyActors (oneshot, then additional/inputrc,
// then the default mapping), mirroring Reline::KeyActor::Composite.
type compositeKeyActor struct {
	actors []*keyActor
}

func (c *compositeKeyActor) matching(key []int) bool {
	for _, a := range c.actors {
		if a != nil && a.matching(key) {
			return true
		}
	}
	return false
}

func (c *compositeKeyActor) get(key []int) (string, bool) {
	for _, a := range c.actors {
		if a == nil {
			continue
		}
		if v, ok := a.get(key); ok {
			return v, true
		}
	}
	return "", false
}
