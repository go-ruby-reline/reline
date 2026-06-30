package reline

// tokenKind classifies a token produced by scanWidth, mirroring the capture
// groups of MRI Reline::Unicode::WIDTH_SCANNER.
type tokenKind int

const (
	tokNonPrintStart tokenKind = iota // \x01
	tokNonPrintEnd                    // \x02
	tokCSI                            // \e[ ... <final letter>
	tokOSC                            // \e] ... (BEL or ESC\)
	tokGrapheme                       // one grapheme cluster
)

// scanWidth walks str emitting tokens in order, matching the alternation in
// MRI's WIDTH_SCANNER: non-printing start/end markers, CSI sequences, OSC
// sequences, then grapheme clusters. The \G anchoring means each position is
// consumed by exactly one alternative.
func scanWidth(str string, emit func(tokenKind, string)) {
	b := []byte(str)
	i := 0
	for i < len(b) {
		switch {
		case b[i] == 0x01:
			emit(tokNonPrintStart, "\x01")
			i++
		case b[i] == 0x02:
			emit(tokNonPrintEnd, "\x02")
			i++
		case b[i] == 0x1b && i+1 < len(b) && b[i+1] == '[':
			if n := matchCSI(b, i); n > 0 {
				emit(tokCSI, string(b[i:i+n]))
				i += n
				continue
			}
			// Lone ESC[ that is not a complete CSI: treat ESC as a grapheme.
			i += emitGrapheme(str, b, i, emit)
		case b[i] == 0x1b && i+1 < len(b) && b[i+1] == ']':
			if n := matchOSC(b, i); n > 0 {
				emit(tokOSC, string(b[i:i+n]))
				i += n
				continue
			}
			i += emitGrapheme(str, b, i, emit)
		default:
			i += emitGrapheme(str, b, i, emit)
		}
	}
}

// emitGrapheme emits the single grapheme cluster starting at byte offset i and
// returns its byte length.
func emitGrapheme(str string, b []byte, i int, emit func(tokenKind, string)) int {
	// b[i:] is non-empty here (the scanWidth loop guarantees i < len(b)), so
	// graphemeClusters always yields at least one cluster (an invalid byte
	// becomes a single RuneError grapheme).
	gcs := graphemeClusters(string(b[i:]))
	emit(tokGrapheme, gcs[0])
	return len(gcs[0])
}

// matchCSI matches /\e\[[\x30-\x3f]*[\x20-\x2f]*[a-zA-Z]/ at b[i:], returning the
// match length or 0.
func matchCSI(b []byte, i int) int {
	j := i + 2 // past ESC [
	for j < len(b) && b[j] >= 0x30 && b[j] <= 0x3f {
		j++
	}
	for j < len(b) && b[j] >= 0x20 && b[j] <= 0x2f {
		j++
	}
	if j < len(b) && ((b[j] >= 'a' && b[j] <= 'z') || (b[j] >= 'A' && b[j] <= 'Z')) {
		return j + 1 - i
	}
	return 0
}

// matchOSC matches /\e\]\d+(?:;[^;\a\e]+)*(?:\a|\e\\)/ at b[i:], returning the
// match length or 0.
func matchOSC(b []byte, i int) int {
	j := i + 2 // past ESC ]
	start := j
	for j < len(b) && b[j] >= '0' && b[j] <= '9' {
		j++
	}
	if j == start {
		return 0 // need at least one digit
	}
	for j < len(b) && b[j] == ';' {
		k := j + 1
		seg := k
		for k < len(b) && b[k] != ';' && b[k] != 0x07 && b[k] != 0x1b {
			k++
		}
		if k == seg {
			break // empty segment: ; not followed by [^;\a\e]+
		}
		j = k
	}
	// terminator: BEL or ESC \
	if j < len(b) && b[j] == 0x07 {
		return j + 1 - i
	}
	if j+1 < len(b) && b[j] == 0x1b && b[j+1] == '\\' {
		return j + 2 - i
	}
	return 0
}
