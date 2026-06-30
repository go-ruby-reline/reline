package reline

import "strings"

// actionState records cross-key state for history-prefix search (MRI
// @prev_action_state / @next_action_state).
type actionState struct {
	typ string
	val string
}

var nullActionState = actionState{}

const maxUndoRedoHistorySize = 100

// undoEntry is one snapshot in the undo/redo ring (buffer, cursor, line index).
type undoEntry struct {
	buffer    []string
	bytePtr   int
	lineIndex int
}

// CompletionProc computes completion candidates for the word `target`, given the
// text before (`pre`) and after (`post`) it. Returning nil means "no completion"
// (MRI's completion_proc that may return non-Array).
type CompletionProc func(target, pre, post string) []string

// LineEditor is the MRI Reline::LineEditor editing state machine: a multiline
// rune-aware buffer with a byte cursor, the full emacs+vi command set, history
// navigation + incremental search, kill-ring, completion, and undo/redo. It is
// driven purely by Keys (Update) and exposes the buffer/cursor for rendering.
type LineEditor struct {
	config *Config
	io     IO
	hist   *History

	bufferOfLines []string
	lineIndex     int
	bytePointer   int

	prompt      string
	isMultiline bool
	finished    bool
	eof         bool

	killRing    *KillRing
	viClipboard string

	historyPointer    int  // -1 means "not in history" (MRI nil)
	hasHistoryPointer bool // false == MRI nil
	lineBackupHistory string
	hasLineBackup     bool
	markPointer       [2]int
	hasMark           bool

	viArg    int
	hasViArg bool

	waitingProc            func(key, sym string)
	viWaitingOperator      string
	viWaitingOperatorArg   int
	hasViWaitingOperator   bool
	dropTerminateSpaces    bool
	inPasting              bool
	continuousInsertBuffer string

	searchingPrompt    string
	hasSearchingPrompt bool

	completionProc       CompletionProc
	completionAppendChar string
	completionState      completionState
	completionOccurs     bool
	perfectMatched       string
	menuInfo             []string
	hasMenu              bool
	completionJourney    *completionJourneyState
	completionIgnoreCase bool

	undoRedoHistory []undoEntry
	undoRedoIndex   int
	restoring       bool

	prevActionState actionState
	nextActionState actionState

	// confirmMultilineTermination decides, given the whole buffer (with a
	// trailing newline), whether Enter should submit (true) or insert a newline
	// (false). When nil in multiline mode, Enter always inserts a newline (so
	// the buffer grows) until the embedder supplies a predicate.
	confirmMultilineTermination func(buffer string) bool

	screenRows, screenCols int
}

type completionState int

const (
	csNormal completionState = iota
	csMenu
	csMenuWithPerfectMatch
	csPerfectMatch
)

type completionJourneyState struct {
	lineIndex int
	pre       string
	target    string
	post      string
	list      []string
	pointer   int
}

// NewLineEditor returns a LineEditor wired to config and the terminal seam io.
// hist is the shared history (Reline::HISTORY).
func NewLineEditor(config *Config, io IO, hist *History) *LineEditor {
	le := &LineEditor{config: config, io: io, hist: hist}
	le.screenRows, le.screenCols = io.GetScreenSize()
	le.resetVariables("")
	return le
}

// Reset re-initializes the editor for a new read with the given prompt and
// refreshes the screen size from the IO seam.
func (le *LineEditor) Reset(prompt string) {
	le.screenRows, le.screenCols = le.io.GetScreenSize()
	le.resetVariables(prompt)
}

func (le *LineEditor) resetVariables(prompt string) {
	le.prompt = strings.ReplaceAll(prompt, "\n", "\\n")
	le.hasMark = false
	le.isMultiline = false
	le.finished = false
	le.hasHistoryPointer = false
	le.historyPointer = -1
	if le.killRing == nil {
		le.killRing = NewKillRing(1024)
	}
	le.viClipboard = ""
	le.hasViArg = false
	le.viArg = 0
	le.waitingProc = nil
	le.hasViWaitingOperator = false
	le.viWaitingOperator = ""
	le.completionJourney = nil
	le.completionState = csNormal
	le.perfectMatched = ""
	le.menuInfo = nil
	le.hasMenu = false
	le.hasSearchingPrompt = false
	le.searchingPrompt = ""
	le.eof = false
	le.continuousInsertBuffer = ""
	le.dropTerminateSpaces = false
	le.inPasting = false
	le.completionIgnoreCase = le.config.CompletionIgnoreCase
	le.undoRedoHistory = []undoEntry{{buffer: []string{""}, bytePtr: 0, lineIndex: 0}}
	le.undoRedoIndex = 0
	le.restoring = false
	le.prevActionState = nullActionState
	le.nextActionState = nullActionState
	le.completionAppendChar = ""
	le.ResetLine()
}

// ResetLine clears the buffer to a single empty line with the cursor at home.
func (le *LineEditor) ResetLine() {
	le.bytePointer = 0
	le.bufferOfLines = []string{""}
	le.lineIndex = 0
	le.hasLineBackup = false
	le.lineBackupHistory = ""
}

// MultilineOn enables multiline editing (Enter inserts a newline until the
// termination predicate accepts).
func (le *LineEditor) MultilineOn() { le.isMultiline = true }

// MultilineOff restricts editing to a single line (Enter finishes).
func (le *LineEditor) MultilineOff() { le.isMultiline = false }

// SetCompletionProc installs the completion candidate generator.
func (le *LineEditor) SetCompletionProc(p CompletionProc) { le.completionProc = p }

// SetCompletionAppendCharacter sets the character appended after a unique
// completion (typically a space).
func (le *LineEditor) SetCompletionAppendCharacter(s string) { le.completionAppendChar = s }

// BytePointer returns the cursor's byte offset within the current line.
func (le *LineEditor) BytePointer() int { return le.bytePointer }

// SetBytePointer sets the cursor's byte offset (MRI byte_pointer=).
func (le *LineEditor) SetBytePointer(v int) { le.bytePointer = v }

// LineIndex returns the index of the current line in a multiline buffer.
func (le *LineEditor) LineIndex() int { return le.lineIndex }

// CurrentLine returns the line the cursor is on.
func (le *LineEditor) CurrentLine() string { return le.bufferOfLines[le.lineIndex] }

// WholeLines returns a copy of every buffer line.
func (le *LineEditor) WholeLines() []string {
	return append([]string(nil), le.bufferOfLines...)
}

// WholeBuffer returns the whole multiline buffer joined by newlines.
func (le *LineEditor) WholeBuffer() string { return strings.Join(le.bufferOfLines, "\n") }

// Line returns the submitted line (whole buffer), or "" with ok=false at EOF
// (MRI LineEditor#line returns nil on eof).
func (le *LineEditor) Line() (string, bool) {
	if le.eof {
		return "", false
	}
	return le.WholeBuffer(), true
}

// Finished reports whether the current read has terminated (Enter or EOF).
func (le *LineEditor) Finished() bool { return le.finished }

// EOF reports whether the read ended via end-of-input (e.g. Ctrl-D on empty).
func (le *LineEditor) EOF() bool { return le.eof }

func (le *LineEditor) finish() {
	le.finished = true
	le.config.reset()
}

// SetConfirmMultilineTermination installs the predicate that decides whether
// Enter on the last line submits the buffer (true) or inserts a newline. The
// embedder (e.g. IRB) supplies one that checks for a complete Ruby expression.
func (le *LineEditor) SetConfirmMultilineTermination(p func(buffer string) bool) {
	le.confirmMultilineTermination = p
}

// confirmMultilineTermination evaluates the termination predicate, defaulting to
// "not terminated" (keep editing) when none is set (MRI requires a proc for
// readmultiline; the pure core treats its absence as never-terminating).
func (le *LineEditor) confirmMultilineTerminationCall() bool {
	if le.confirmMultilineTermination == nil {
		return false
	}
	return le.confirmMultilineTermination(le.WholeBuffer() + "\n")
}

func (le *LineEditor) bufferEmpty() bool {
	return le.CurrentLine() == "" && len(le.bufferOfLines) == 1
}

func (le *LineEditor) setCurrentLine(line string, bytePtr int, hasBytePtr bool) {
	cursor := le.currentBytePointerCursor()
	le.bufferOfLines[le.lineIndex] = line
	if hasBytePtr {
		le.bytePointer = bytePtr
	} else {
		le.calculateNearestCursor(cursor)
	}
}

func (le *LineEditor) currentBytePointerCursor() int {
	return CalculateWidth(le.CurrentLine()[:le.bytePointer], false)
}

func (le *LineEditor) calculateNearestCursor(cursor int) {
	line := le.CurrentLine()
	newCursorMax := CalculateWidth(line, false)
	endOfLineCursor := newCursorMax
	if le.config.EditingModeIs(ModeViCommand) {
		lastByteSize := getPrevMbcharSize(line, len(line))
		if lastByteSize > 0 {
			lastMbchar := line[len(line)-lastByteSize:]
			endOfLineCursor = newCursorMax - getMbcharWidth(lastMbchar)
		}
	}
	newCursor := 0
	newBytePointer := 0
	for _, gc := range graphemeClusters(line) {
		w := getMbcharWidth(gc)
		now := newCursor + w
		if now > endOfLineCursor || now > cursor {
			break
		}
		newCursor += w
		newBytePointer += len(gc)
	}
	le.bytePointer = newBytePointer
}

func byteslice(str string, off, size int) string {
	if off < 0 || off > len(str) {
		return ""
	}
	end := off + size
	if end > len(str) {
		end = len(str)
	}
	if end < off {
		end = off
	}
	return str[off:end]
}

// byteslicePop removes size bytes at off, returning (remaining, removed).
func byteslicePop(str string, off, size int) (string, string) {
	newStr := str[:off] + byteslice(str, off+size, len(str))
	return newStr, byteslice(str, off, size)
}

func byteinsert(str string, off int, other string) string {
	return str[:off] + other + str[off:]
}

// InsertText inserts text at the cursor (MRI insert_text), advancing the cursor.
func (le *LineEditor) InsertText(text string) {
	if len(le.bufferOfLines[le.lineIndex]) == le.bytePointer {
		le.bufferOfLines[le.lineIndex] += text
	} else {
		le.bufferOfLines[le.lineIndex] = byteinsert(le.bufferOfLines[le.lineIndex], le.bytePointer, text)
	}
	le.bytePointer += len(text)
}

// InsertMultilineText inserts text that may contain newlines, splitting the
// current line (MRI insert_multiline_text).
func (le *LineEditor) InsertMultilineText(text string) {
	pre := byteslice(le.bufferOfLines[le.lineIndex], 0, le.bytePointer)
	post := le.bufferOfLines[le.lineIndex][le.bytePointer:]
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	// strings.Split never returns an empty slice, so lines has at least one row.
	lines := strings.Split(pre+text+post, "\n")
	tail := append([]string{}, le.bufferOfLines[le.lineIndex+1:]...)
	le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex], append(lines, tail...)...)
	le.lineIndex += len(lines) - 1
	le.bytePointer = len(le.bufferOfLines[le.lineIndex]) - len(post)
}

// DeleteText deletes a range of the current line, or (with hasArgs=false)
// deletes the current line / merges per MRI delete_text() with no args.
func (le *LineEditor) DeleteText(start, length int, hasArgs bool) {
	if !hasArgs {
		switch {
		case len(le.bufferOfLines) == 1:
			le.bufferOfLines[le.lineIndex] = ""
			le.bytePointer = 0
		case le.lineIndex == len(le.bufferOfLines)-1 && le.lineIndex > 0:
			le.bufferOfLines = le.bufferOfLines[:len(le.bufferOfLines)-1]
			le.lineIndex--
			le.bytePointer = 0
		case le.lineIndex < len(le.bufferOfLines)-1:
			le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex], le.bufferOfLines[le.lineIndex+1:]...)
			le.bytePointer = 0
		}
		return
	}
	cur := le.CurrentLine()
	before := byteslice(cur, 0, start)
	after := byteslice(cur, start+length, len(cur))
	le.setCurrentLine(before+after, 0, false)
}

// Update feeds one key event through the dispatch + post-processing pipeline and
// returns whether the buffer changed (MRI update/input_key).
func (le *LineEditor) Update(key Key) bool {
	return le.inputKey(key)
}

func (le *LineEditor) inputKey(key Key) bool {
	old := le.WholeLines()
	le.config.resetOneshotKeyBindings()
	if key.EOF {
		le.processInsert(true)
		le.eof = le.bufferEmpty()
		le.finish()
		return false
	}

	le.completionOccurs = false
	le.processKey(key.Char, key.MethodSymbol)

	if le.config.EditingModeIs(ModeViCommand) && le.bytePointer > 0 && le.bytePointer == len(le.CurrentLine()) {
		byteSize := getPrevMbcharSize(le.bufferOfLines[le.lineIndex], le.bytePointer)
		le.bytePointer -= byteSize
	}

	le.prevActionState, le.nextActionState = le.nextActionState, nullActionState

	if !le.completionOccurs {
		le.completionState = csNormal
		le.completionJourney = nil
	}

	modified := !equalLines(old, le.bufferOfLines)
	if !le.restoring {
		le.pushUndoRedo(modified)
	}
	le.restoring = false

	if !le.completionOccurs && modified && !le.config.DisableCompletion && le.config.Autocompletion {
		le.processInsert(true)
		le.completionJourney = le.retrieveCompletionJourneyState()
	}
	return modified
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (le *LineEditor) pushUndoRedo(modified bool) {
	snap := undoEntry{buffer: le.WholeLines(), bytePtr: le.bytePointer, lineIndex: le.lineIndex}
	if modified {
		le.undoRedoHistory = append(le.undoRedoHistory[:le.undoRedoIndex+1], snap)
		if len(le.undoRedoHistory) > maxUndoRedoHistorySize {
			le.undoRedoHistory = le.undoRedoHistory[1:]
		}
		le.undoRedoIndex = len(le.undoRedoHistory) - 1
	} else {
		le.undoRedoHistory[le.undoRedoIndex] = snap
	}
}

var viWaitingAcceptMethods = map[string]bool{
	"vi_change_meta": true, "vi_delete_meta": true, "vi_yank": true,
	"ed_insert": true, "ed_argument_digit": true,
}

var viMotions = map[string]bool{
	"ed_prev_char": true, "ed_next_char": true, "vi_zero": true,
	"ed_move_to_beg": true, "ed_move_to_end": true, "vi_to_column": true,
	"vi_next_char": true, "vi_prev_char": true, "vi_next_word": true,
	"vi_prev_word": true, "vi_to_next_char": true, "vi_to_prev_char": true,
	"vi_end_word": true, "vi_next_big_word": true, "vi_prev_big_word": true,
	"vi_end_big_word": true,
}

var argumentDigitMethods = map[string]bool{
	"ed_digit": true, "vi_zero": true, "ed_argument_digit": true,
}

func (le *LineEditor) processKey(key, methodSymbol string) {
	if le.waitingProc != nil {
		if charCount(key) != 1 {
			le.cleanupWaiting()
		}
	}
	if le.hasViWaitingOperator {
		if !viWaitingAcceptMethods[methodSymbol] && !viMotions[methodSymbol] {
			le.cleanupWaiting()
		}
	}
	le.processInsert(methodSymbol != "ed_insert")
	le.runForOperators(key, methodSymbol)
}

func (le *LineEditor) runForOperators(key, methodSymbol string) {
	if methodSymbol == "ed_insert" && le.config.EditingModeIs(ModeViCommand) && le.waitingProc == nil {
		return
	}
	if argumentDigitMethods[methodSymbol] && le.waitingProc == nil {
		// MRI returns here WITHOUT resetting @vi_arg, so the accumulated
		// numeric argument carries to the next command.
		le.wrapMethodCall(methodSymbol, key, false)
		return
	}
	if le.hasViWaitingOperator {
		if le.waitingProc != nil || viMotions[methodSymbol] {
			oldBytePointer := le.bytePointer
			arg := 1
			if le.hasViArg {
				arg = le.viArg
			}
			le.viArg = arg * le.viWaitingOperatorArg
			le.hasViArg = true
			le.wrapMethodCall(methodSymbol, key, true)
			if le.waitingProc == nil {
				diff := le.bytePointer - oldBytePointer
				le.bytePointer = oldBytePointer
				le.dispatchOperator(le.viWaitingOperator, diff)
				le.cleanupWaiting()
			}
		} else {
			le.wrapMethodCall(methodSymbol, key, false)
			le.cleanupWaiting()
		}
	} else {
		le.wrapMethodCall(methodSymbol, key, false)
	}
	le.hasViArg = false
	le.killRing.Process()
}

func (le *LineEditor) cleanupWaiting() {
	le.waitingProc = nil
	le.hasViWaitingOperator = false
	le.viWaitingOperator = ""
	le.hasSearchingPrompt = false
	le.searchingPrompt = ""
	le.dropTerminateSpaces = false
}

func (le *LineEditor) processInsert(force bool) {
	if le.continuousInsertBuffer == "" || (le.inPasting && !force) {
		return
	}
	le.InsertText(le.continuousInsertBuffer)
	le.continuousInsertBuffer = ""
}

// SetPastingState toggles bracketed-paste batching: text typed while pasting is
// accumulated and force-inserted on exit (MRI set_pasting_state).
func (le *LineEditor) SetPastingState(inPasting bool) {
	if le.inPasting && !inPasting {
		le.processInsert(true)
	}
	le.inPasting = inPasting
}

func charCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
