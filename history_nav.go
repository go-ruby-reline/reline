package reline

import "strings"

// line/cursor placement selectors for moveHistory (MRI :start / :end / index).
type linePlacement int

const (
	lineStart   linePlacement = -1
	lineEnd     linePlacement = -2
	lineLiteral linePlacement = 0 // use literalLine arg
)

type cursorPlacement int

const (
	cursorStart   cursorPlacement = -1
	cursorEnd     cursorPlacement = -2
	cursorLiteral cursorPlacement = 0 // use literalCursor arg
)

func hasPtr(b bool) bool { return b }

// moveHistory loads the history entry at historyPointer into the buffer,
// stashing the current line for round-trip (MRI move_history). historyPointer ==
// HISTORY.size restores the in-progress backup. lp/cp select where the cursor
// and line index land; literal* supply explicit values when *Literal is used.
func (le *LineEditor) moveHistory(historyPointer int, hasHistoryPointer bool, lp linePlacement, cp cursorPlacement, literalLine, literalCursor int) {
	if !hasHistoryPointer {
		historyPointer = le.hist.Size()
	}
	if historyPointer < 0 || historyPointer > le.hist.Size() {
		return
	}
	oldHistoryPointer := le.hist.Size()
	if le.hasHistoryPointer {
		oldHistoryPointer = le.historyPointer
	}
	if oldHistoryPointer == le.hist.Size() {
		le.lineBackupHistory = le.WholeBuffer()
		le.hasLineBackup = true
	} else {
		le.hist.setAt(oldHistoryPointer, le.WholeBuffer())
	}
	var buf string
	if historyPointer == le.hist.Size() {
		buf = le.lineBackupHistory
		le.hasHistoryPointer = false
		le.historyPointer = -1
		le.hasLineBackup = false
		le.lineBackupHistory = ""
	} else {
		buf = le.hist.at(historyPointer)
		le.historyPointer = historyPointer
		le.hasHistoryPointer = true
	}
	// strings.Split never returns an empty slice (an empty buf yields [""]).
	le.bufferOfLines = strings.Split(buf, "\n")
	switch lp {
	case lineStart:
		le.lineIndex = 0
	case lineEnd:
		le.lineIndex = len(le.bufferOfLines) - 1
	default:
		le.lineIndex = literalLine
	}
	switch cp {
	case cursorStart:
		le.bytePointer = 0
	case cursorEnd:
		le.bytePointer = len(le.CurrentLine())
	default:
		le.bytePointer = literalCursor
	}
}

func (le *LineEditor) edPrevHistory(c cmdCtx) {
	arg := c.arg
	for {
		if le.lineIndex > 0 {
			cursor := le.currentBytePointerCursor()
			le.lineIndex--
			le.calculateNearestCursor(cursor)
			return
		}
		ptr := le.hist.Size()
		if le.hasHistoryPointer {
			ptr = le.historyPointer
		}
		cp := cursorEnd
		if le.config.EditingModeIs(ModeViCommand) {
			cp = cursorStart
		}
		le.moveHistory(ptr-1, true, lineEnd, cp, 0, 0)
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) edNextHistory(c cmdCtx) {
	arg := c.arg
	for {
		if le.lineIndex < len(le.bufferOfLines)-1 {
			cursor := le.currentBytePointerCursor()
			le.lineIndex++
			le.calculateNearestCursor(cursor)
			return
		}
		ptr := le.hist.Size()
		if le.hasHistoryPointer {
			ptr = le.historyPointer
		}
		cp := cursorEnd
		if le.config.EditingModeIs(ModeViCommand) {
			cp = cursorStart
		}
		le.moveHistory(ptr+1, true, lineStart, cp, 0, 0)
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) edBeginningOfHistory(cmdCtx) {
	le.moveHistory(0, true, lineEnd, cursorEnd, 0, 0)
}
func (le *LineEditor) edEndOfHistory(cmdCtx) {
	le.moveHistory(le.hist.Size(), true, lineEnd, cursorEnd, 0, 0)
}

// searchHistory finds the first history entry (within pointers) whose line
// starts with prefix (MRI search_history).
func (le *LineEditor) searchHistory(prefix string, pointers []int) (int, int, bool) {
	for _, pointer := range pointers {
		entry := le.hist.at(pointer)
		for index, line := range strings.Split(entry, "\n") {
			if strings.HasPrefix(line, prefix) {
				return pointer, index, true
			}
		}
	}
	return 0, 0, false
}

func (le *LineEditor) edSearchPrevHistory(c cmdCtx) {
	arg := c.arg
	for {
		substr := byteslice(le.CurrentLine(), 0, le.bytePointer)
		if le.prevActionState.typ == "search_history" && le.prevActionState.val == "empty" {
			substr = ""
		}
		if le.hasHistoryPointer && le.historyPointer == 0 {
			return
		}
		if !le.hasHistoryPointer && substr == "" && le.CurrentLine() != "" {
			return
		}
		upper := le.hist.Size()
		if le.hasHistoryPointer {
			upper = le.historyPointer
		}
		pointers := descending(upper)
		hPtr, lineIndex, ok := le.searchHistory(substr, pointers)
		if !ok {
			return
		}
		lp := lineLiteral
		cp := cursorLiteral
		litCursor := le.bytePointer
		if substr == "" {
			cp = cursorEnd
		}
		le.moveHistory(hPtr, true, lp, cp, lineIndex, litCursor)
		arg--
		if substr == "" {
			le.nextActionState = actionState{"search_history", "empty"}
		}
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) edSearchNextHistory(c cmdCtx) {
	arg := c.arg
	for {
		substr := byteslice(le.CurrentLine(), 0, le.bytePointer)
		if le.prevActionState.typ == "search_history" && le.prevActionState.val == "empty" {
			substr = ""
		}
		if !le.hasHistoryPointer {
			return
		}
		pointers := ascending(le.historyPointer+1, le.hist.Size())
		hPtr, lineIndex, ok := le.searchHistory(substr, pointers)
		if !ok && substr != "" {
			return
		}
		ptr := hPtr
		hasP := ok
		lp := lineLiteral
		cp := cursorLiteral
		litCursor := le.bytePointer
		if substr == "" {
			cp = cursorEnd
		}
		if !hasP {
			// move_history(nil) -> restore backup at HISTORY.size
			le.moveHistory(le.hist.Size(), true, lineStart, cp, 0, litCursor)
		} else {
			le.moveHistory(ptr, true, lp, cp, lineIndex, litCursor)
		}
		arg--
		if substr == "" {
			le.nextActionState = actionState{"search_history", "empty"}
		}
		if arg <= 0 {
			break
		}
	}
}

func descending(upper int) []int {
	out := make([]int, 0, upper)
	for i := upper - 1; i >= 0; i-- {
		out = append(out, i)
	}
	return out
}

func ascending(from, to int) []int {
	out := make([]int, 0)
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}
