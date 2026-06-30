// Package reline is a pure-Go (CGO=0) port of MRI 4.0.5's Reline line-editor
// core: the editing state machine (line buffer, cursor, keymap dispatch,
// editing commands, history, kill-ring, completion) plus the pure rendering
// computation. The terminal raw-mode I/O is a host seam (see io.go) wired by
// the embedder (e.g. rbgo for a pure-Go IRB).
package reline

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// AmbiguousWidth is the display width used for East-Asian "ambiguous" width
// characters. MRI resolves this at runtime from the terminal probe; the default
// is 1 (mirrors Reline.ambiguous_width when STDOUT is not a tty / dumb).
var AmbiguousWidth = 1

// escapedPairs mirrors Reline::Unicode::EscapedPairs: control characters that
// print as a two-cell caret escape (e.g. 0x01 -> "^A").
var escapedPairs = map[rune]string{
	0x00: "^@", 0x01: "^A", 0x02: "^B", 0x03: "^C", 0x04: "^D", 0x05: "^E",
	0x06: "^F", 0x07: "^G", 0x08: "^H", 0x09: "^I", 0x0A: "^J", 0x0B: "^K",
	0x0C: "^L", 0x0D: "^M", 0x0E: "^N", 0x0F: "^O", 0x10: "^P", 0x11: "^Q",
	0x12: "^R", 0x13: "^S", 0x14: "^T", 0x15: "^U", 0x16: "^V", 0x17: "^W",
	0x18: "^X", 0x19: "^Y", 0x1A: "^Z", 0x1B: "^[", 0x1C: "^\\", 0x1D: "^]",
	0x1E: "^^", 0x1F: "^_", 0x7F: "^?",
}

// eastAsianWidth returns the display width of a codepoint per the EAW chunk
// table (MRI Reline::Unicode.east_asian_width).
func eastAsianWidth(ord int) int {
	// bsearch_index { |o| ord <= o } : first chunk whose last >= ord. The table
	// covers the full code-point range (final chunk's last is U+10FFFF), so the
	// search always lands in range.
	idx := sort.Search(len(eawChunkLast), func(i int) bool { return ord <= eawChunkLast[i] })
	w := eawChunkWidth[idx]
	if w == -1 {
		return AmbiguousWidth
	}
	return w
}

// graphemeClusters splits s into grapheme clusters. This implements the subset
// of UAX#29 that Reline relies on: combining marks attach to the preceding base
// (zero width), ZWJ joins, and regional-indicator pairs form flags. ASCII fast
// path covers the dominant IRB case.
func graphemeClusters(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var out []string
	i := 0
	for i < len(runes) {
		start := i
		i++
		for i < len(runes) {
			r := runes[i]
			prev := runes[i-1]
			if r == 0x200D { // ZWJ: join and continue
				i++
				if i < len(runes) {
					i++
				}
				continue
			}
			if isExtend(r) {
				i++
				continue
			}
			if isRegionalIndicator(prev) && isRegionalIndicator(r) {
				// Form a flag from exactly two regional indicators.
				i++
				break
			}
			break
		}
		out = append(out, string(runes[start:i]))
	}
	return out
}

func isExtend(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me) || r == 0xFE0F || r == 0xFE0E
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

// firstRune returns the first rune of s (its ordinal), like Ruby String#ord.
func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// getMbcharWidth returns the display width of a single grapheme cluster
// (MRI Reline::Unicode.get_mbchar_width).
func getMbcharWidth(mbchar string) int {
	ord := firstRune(mbchar)
	if ord <= 0x1F {
		return 2
	}
	if utf8.RuneCountInString(mbchar) == 1 && ord <= 0x7E {
		return 1
	}
	zwj := false
	width := 0
	for _, c := range mbchar {
		if zwj {
			zwj = false
			continue
		}
		if c == 0x200D {
			zwj = true
			continue
		}
		width += eastAsianWidth(int(c))
	}
	return width
}

// CalculateWidth returns the display width of str. When allowEscapeCode is true,
// CSI/OSC escape sequences and non-printing markers are excluded from the count
// (MRI Reline::Unicode.calculate_width).
func CalculateWidth(str string, allowEscapeCode bool) int {
	if !allowEscapeCode {
		w := 0
		for _, gc := range graphemeClusters(str) {
			w += getMbcharWidth(gc)
		}
		return w
	}
	width := 0
	inZeroWidth := false
	scanWidth(str, func(t tokenKind, s string) {
		switch t {
		case tokNonPrintStart:
			inZeroWidth = true
		case tokNonPrintEnd:
			inZeroWidth = false
		case tokCSI, tokOSC:
			// excluded
		case tokGrapheme:
			if !inZeroWidth {
				width += getMbcharWidth(s)
			}
		}
	})
	return width
}

// SplitByWidth wraps str into display lines no wider than maxWidth and returns
// the lines plus their count (MRI Reline::Unicode.split_by_width, used by IRB).
func SplitByWidth(str string, maxWidth int) ([]string, int) {
	lines := SplitLineByWidth(str, maxWidth, 0)
	return lines, len(lines)
}

// SplitLineByWidth wraps str into display lines no wider than maxWidth, starting
// at the given column offset, carrying CSI color state across wraps
// (MRI Reline::Unicode.split_line_by_width).
func SplitLineByWidth(str string, maxWidth, offset int) []string {
	lines := []string{""}
	width := offset
	inZeroWidth := false
	seq := ""
	scanWidth(str, func(t tokenKind, s string) {
		switch t {
		case tokNonPrintStart:
			inZeroWidth = true
		case tokNonPrintEnd:
			inZeroWidth = false
		case tokCSI:
			lines[len(lines)-1] += s
			if !inZeroWidth {
				if s == "\x1b[m" || s == "\x1b[0m" {
					seq = ""
				} else {
					seq += s
				}
			}
		case tokOSC:
			lines[len(lines)-1] += s
			if !inZeroWidth {
				seq += s
			}
		case tokGrapheme:
			if !inZeroWidth {
				mw := getMbcharWidth(s)
				width += mw
				if width > maxWidth {
					width = mw
					lines = append(lines, seq)
				}
			}
			lines[len(lines)-1] += s
		}
	})
	if width == maxWidth {
		lines = append(lines, "")
	}
	return lines
}

// EscapeForPrint converts control characters to their printable caret escapes,
// keeping newline and expanding tab to two spaces (MRI escape_for_print).
func EscapeForPrint(str string) string {
	var b strings.Builder
	for _, gc := range graphemeClusters(str) {
		switch gc {
		case "\n":
			b.WriteString(gc)
		case "\t":
			b.WriteString("  ")
		default:
			if rep, ok := escapedPairs[firstRune(gc)]; ok && utf8.RuneCountInString(gc) == 1 {
				b.WriteString(rep)
			} else {
				b.WriteString(gc)
			}
		}
	}
	return b.String()
}

// getNextMbcharSize returns the byte size of the grapheme cluster starting at
// bytePointer (MRI get_next_mbchar_size).
func getNextMbcharSize(line string, bytePointer int) int {
	if bytePointer >= len(line) {
		return 0
	}
	// line[bytePointer:] is non-empty here, so there is at least one cluster.
	return len(graphemeClusters(line[bytePointer:])[0])
}

// getPrevMbcharSize returns the byte size of the grapheme cluster ending at
// bytePointer (MRI get_prev_mbchar_size).
func getPrevMbcharSize(line string, bytePointer int) int {
	if bytePointer <= 0 {
		return 0
	}
	// line[:bytePointer] is non-empty here, so there is at least one cluster.
	gcs := graphemeClusters(line[:bytePointer])
	return len(gcs[len(gcs)-1])
}

func sumBytes(gcs []string) int {
	n := 0
	for _, g := range gcs {
		n += len(g)
	}
	return n
}

func takeWhile(gcs []string, pred func(string) bool) []string {
	var out []string
	for _, g := range gcs {
		if !pred(g) {
			break
		}
		out = append(out, g)
	}
	return out
}

func reverseGCs(gcs []string) []string {
	out := make([]string, len(gcs))
	for i, g := range gcs {
		out[len(gcs)-1-i] = g
	}
	return out
}

// wordCharacter reports whether the grapheme is a word character (\p{Word}).
func wordCharacter(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.In(r, unicode.Mn, unicode.Mc, unicode.Pc) || r == '_') {
			return false
		}
	}
	return true
}

// spaceCharacter reports whether the grapheme matches \s.
func spaceCharacter(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v':
		default:
			return false
		}
	}
	return true
}

func emForwardWord(line string, bytePointer int) int {
	gcs := graphemeClusters(line[bytePointer:])
	nonwords := takeWhile(gcs, func(c string) bool { return !wordCharacter(c) })
	words := takeWhile(gcs[len(nonwords):], wordCharacter)
	return sumBytes(nonwords) + sumBytes(words)
}

func emForwardWordWithCapitalization(line string, bytePointer int) (int, string) {
	gcs := graphemeClusters(line[bytePointer:])
	nonwords := takeWhile(gcs, func(c string) bool { return !wordCharacter(c) })
	words := takeWhile(gcs[len(nonwords):], wordCharacter)
	return sumBytes(nonwords) + sumBytes(words), strings.Join(nonwords, "") + capitalize(strings.Join(words, ""))
}

func emBackwardWord(line string, bytePointer int) int {
	gcs := reverseGCs(graphemeClusters(line[:bytePointer]))
	nonwords := takeWhile(gcs, func(c string) bool { return !wordCharacter(c) })
	words := takeWhile(gcs[len(nonwords):], wordCharacter)
	return sumBytes(nonwords) + sumBytes(words)
}

func emBigBackwardWord(line string, bytePointer int) int {
	gcs := reverseGCs(graphemeClusters(line[:bytePointer]))
	spaces := takeWhile(gcs, spaceCharacter)
	nonspaces := takeWhile(gcs[len(spaces):], func(c string) bool { return !spaceCharacter(c) })
	return sumBytes(spaces) + sumBytes(nonspaces)
}

// edTransposeWords returns the four byte offsets that delimit the two words to
// be transposed (MRI Reline::Unicode.ed_transpose_words).
func edTransposeWords(line string, bytePointer int) (int, int, int, int) {
	before := graphemeClusters(line[:bytePointer])
	pos := len(before)
	gcs := append(append([]string{}, before...), graphemeClusters(line[bytePointer:])...)
	for pos < len(gcs) && !wordCharacter(gcs[pos]) {
		pos++
	}
	var secondWordEnd int
	if pos == len(gcs) {
		for pos > 0 && !wordCharacter(gcs[pos-1]) {
			pos--
		}
		secondWordEnd = len(gcs)
	} else {
		for pos < len(gcs) && wordCharacter(gcs[pos]) {
			pos++
		}
		secondWordEnd = pos
	}
	for pos > 0 && wordCharacter(gcs[pos-1]) {
		pos--
	}
	secondWordStart := pos
	for pos > 0 && !wordCharacter(gcs[pos-1]) {
		pos--
	}
	firstWordEnd := pos
	for pos > 0 && wordCharacter(gcs[pos-1]) {
		pos--
	}
	firstWordStart := pos
	conv := func(idx int) int { return sumBytes(gcs[:idx]) }
	return conv(firstWordStart), conv(firstWordEnd), conv(secondWordStart), conv(secondWordEnd)
}

func viBigForwardWord(line string, bytePointer int) int {
	gcs := graphemeClusters(line[bytePointer:])
	nonspaces := takeWhile(gcs, func(c string) bool { return !spaceCharacter(c) })
	spaces := takeWhile(gcs[len(nonspaces):], spaceCharacter)
	return sumBytes(nonspaces) + sumBytes(spaces)
}

func viBigForwardEndWord(line string, bytePointer int) int {
	gcs := graphemeClusters(line[bytePointer:])
	if len(gcs) == 0 {
		return 0
	}
	first := gcs[:1]
	gcs = gcs[1:]
	spaces := takeWhile(gcs, spaceCharacter)
	nonspaces := takeWhile(gcs[len(spaces):], func(c string) bool { return !spaceCharacter(c) })
	matched := append(append([]string{}, spaces...), nonspaces...)
	if len(matched) > 0 {
		matched = matched[:len(matched)-1]
	}
	return sumBytes(first) + sumBytes(matched)
}

func viBigBackwardWord(line string, bytePointer int) int {
	gcs := reverseGCs(graphemeClusters(line[:bytePointer]))
	spaces := takeWhile(gcs, spaceCharacter)
	nonspaces := takeWhile(gcs[len(spaces):], func(c string) bool { return !spaceCharacter(c) })
	return sumBytes(spaces) + sumBytes(nonspaces)
}

func viForwardWord(line string, bytePointer int, dropTerminateSpaces bool) int {
	gcs := graphemeClusters(line[bytePointer:])
	if len(gcs) == 0 {
		return 0
	}
	c := gcs[0]
	var matched []string
	switch {
	case wordCharacter(c):
		matched = takeWhile(gcs, wordCharacter)
	case spaceCharacter(c):
		matched = takeWhile(gcs, spaceCharacter)
	default:
		matched = takeWhile(gcs, func(c string) bool { return !wordCharacter(c) && !spaceCharacter(c) })
	}
	if dropTerminateSpaces {
		return sumBytes(matched)
	}
	spaces := takeWhile(gcs[len(matched):], spaceCharacter)
	return sumBytes(matched) + sumBytes(spaces)
}

func viForwardEndWord(line string, bytePointer int) int {
	gcs := graphemeClusters(line[bytePointer:])
	if len(gcs) == 0 {
		return 0
	}
	if len(gcs) == 1 {
		return len(gcs[0])
	}
	start := gcs[0]
	gcs = gcs[1:]
	skips := []string{start}
	if spaceCharacter(start) || (len(gcs) > 0 && spaceCharacter(gcs[0])) {
		spaces := takeWhile(gcs, spaceCharacter)
		skips = append(skips, spaces...)
		gcs = gcs[len(spaces):]
	}
	var startWithWord bool
	if len(gcs) > 0 {
		startWithWord = wordCharacter(gcs[0])
	}
	matched := takeWhile(gcs, func(c string) bool {
		if startWithWord {
			return wordCharacter(c)
		}
		return !wordCharacter(c) && !spaceCharacter(c)
	})
	if len(matched) > 0 {
		matched = matched[:len(matched)-1]
	}
	return sumBytes(skips) + sumBytes(matched)
}

func viBackwardWord(line string, bytePointer int) int {
	gcs := reverseGCs(graphemeClusters(line[:bytePointer]))
	spaces := takeWhile(gcs, spaceCharacter)
	gcs = gcs[len(spaces):]
	var startWithWord bool
	if len(gcs) > 0 {
		startWithWord = wordCharacter(gcs[0])
	}
	matched := takeWhile(gcs, func(c string) bool {
		if startWithWord {
			return wordCharacter(c)
		}
		return !wordCharacter(c) && !spaceCharacter(c)
	})
	return sumBytes(spaces) + sumBytes(matched)
}

// CommonPrefix returns the longest common grapheme-cluster prefix of list,
// optionally case-insensitively (MRI Reline::Unicode.common_prefix).
func CommonPrefix(list []string, ignoreCase bool) string {
	if len(list) == 0 {
		return ""
	}
	common := graphemeClusters(list[0])
	for _, item := range list {
		gcs := graphemeClusters(item)
		var next []string
		for i, gc := range common {
			if i >= len(gcs) {
				break
			}
			if ignoreCase {
				if !strings.EqualFold(gc, gcs[i]) {
					break
				}
			} else if gc != gcs[i] {
				break
			}
			next = append(next, gc)
		}
		common = next
	}
	return strings.Join(common, "")
}

func viFirstPrint(line string) int {
	gcs := graphemeClusters(line)
	spaces := takeWhile(gcs, spaceCharacter)
	return sumBytes(spaces)
}

// capitalize mirrors Ruby String#capitalize: first letter up, rest down.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	out := make([]rune, len(r))
	out[0] = unicode.ToUpper(r[0])
	for i := 1; i < len(r); i++ {
		out[i] = unicode.ToLower(r[i])
	}
	return string(out)
}
