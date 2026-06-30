package reline

import "strings"

func (le *LineEditor) viNextWord(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.CurrentLine()) > le.bytePointer {
			le.bytePointer += viForwardWord(le.CurrentLine(), le.bytePointer, le.dropTerminateSpaces)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viPrevWord(c cmdCtx) {
	arg := c.arg
	for {
		if le.bytePointer > 0 {
			le.bytePointer -= viBackwardWord(le.CurrentLine(), le.bytePointer)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viEndWord(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.CurrentLine()) > le.bytePointer {
			le.bytePointer += viForwardEndWord(le.CurrentLine(), le.bytePointer)
		}
		arg--
		if c.inclusive && arg == 0 {
			byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
			if byteSize > 0 {
				le.bytePointer += byteSize
			}
		}
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viNextBigWord(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.CurrentLine()) > le.bytePointer {
			le.bytePointer += viBigForwardWord(le.CurrentLine(), le.bytePointer)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viPrevBigWord(c cmdCtx) {
	arg := c.arg
	for {
		if le.bytePointer > 0 {
			le.bytePointer -= viBigBackwardWord(le.CurrentLine(), le.bytePointer)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viEndBigWord(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.CurrentLine()) > le.bytePointer {
			le.bytePointer += viBigForwardEndWord(le.CurrentLine(), le.bytePointer)
		}
		arg--
		if c.inclusive && arg == 0 {
			byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
			if byteSize > 0 {
				le.bytePointer += byteSize
			}
		}
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viDeletePrevChar(cmdCtx) {
	if le.bytePointer == 0 && le.lineIndex > 0 {
		le.bytePointer = len(le.bufferOfLines[le.lineIndex-1])
		le.bufferOfLines[le.lineIndex-1] += le.bufferOfLines[le.lineIndex]
		le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex], le.bufferOfLines[le.lineIndex+1:]...)
		le.lineIndex--
	} else if le.bytePointer > 0 {
		byteSize := getPrevMbcharSize(le.CurrentLine(), le.bytePointer)
		le.bytePointer -= byteSize
		line, _ := byteslicePop(le.CurrentLine(), le.bytePointer, byteSize)
		le.setCurrentLine(line, 0, false)
	}
}

func (le *LineEditor) edDeletePrevChar(c cmdCtx) {
	deleted := ""
	for i := 0; i < c.arg; i++ {
		if le.bytePointer > 0 {
			byteSize := getPrevMbcharSize(le.CurrentLine(), le.bytePointer)
			le.bytePointer -= byteSize
			line, mbchar := byteslicePop(le.CurrentLine(), le.bytePointer, byteSize)
			le.setCurrentLine(line, 0, false)
			deleted = mbchar + deleted
		}
	}
	le.copyForVi(deleted)
}

func (le *LineEditor) copyForVi(text string) {
	if le.config.EditingModeIs(ModeViInsert) || le.config.EditingModeIs(ModeViCommand) {
		le.viClipboard = text
	}
}

func (le *LineEditor) viChangeMeta(c cmdCtx) {
	if le.hasViWaitingOperator {
		if le.viWaitingOperator == "vi_change_meta_confirm" && !c.hasArg {
			le.setCurrentLine("", 0, true)
		}
		le.hasViWaitingOperator = false
		le.viWaitingOperator = ""
	} else {
		le.dropTerminateSpaces = true
		le.viWaitingOperator = "vi_change_meta_confirm"
		le.hasViWaitingOperator = true
		le.viWaitingOperatorArg = orOne(c)
	}
}

func (le *LineEditor) viChangeMetaConfirm(diff int) {
	le.viDeleteMetaConfirm(diff)
	le.config.SetEditingMode(ModeViInsert)
	le.dropTerminateSpaces = false
}

func (le *LineEditor) viDeleteMeta(c cmdCtx) {
	if le.hasViWaitingOperator {
		if le.viWaitingOperator == "vi_delete_meta_confirm" && !c.hasArg {
			le.setCurrentLine("", 0, true)
		}
		le.hasViWaitingOperator = false
		le.viWaitingOperator = ""
	} else {
		le.viWaitingOperator = "vi_delete_meta_confirm"
		le.hasViWaitingOperator = true
		le.viWaitingOperatorArg = orOne(c)
	}
}

func (le *LineEditor) viDeleteMetaConfirm(diff int) {
	var line, cut string
	switch {
	case diff > 0:
		line, cut = byteslicePop(le.CurrentLine(), le.bytePointer, diff)
	case diff < 0:
		line, cut = byteslicePop(le.CurrentLine(), le.bytePointer+diff, -diff)
	default:
		return
	}
	le.copyForVi(cut)
	off := 0
	if diff < 0 {
		off = diff
	}
	le.setCurrentLine(line, le.bytePointer+off, true)
}

func (le *LineEditor) viYank(c cmdCtx) {
	if le.hasViWaitingOperator {
		if le.viWaitingOperator == "vi_yank_confirm" && !c.hasArg {
			le.copyForVi(le.CurrentLine())
		}
		le.hasViWaitingOperator = false
		le.viWaitingOperator = ""
	} else {
		le.viWaitingOperator = "vi_yank_confirm"
		le.hasViWaitingOperator = true
		le.viWaitingOperatorArg = orOne(c)
	}
}

func (le *LineEditor) viYankConfirm(diff int) {
	var cut string
	switch {
	case diff > 0:
		cut = byteslice(le.CurrentLine(), le.bytePointer, diff)
	case diff < 0:
		cut = byteslice(le.CurrentLine(), le.bytePointer+diff, -diff)
	default:
		return
	}
	le.copyForVi(cut)
}

func orOne(c cmdCtx) int {
	if c.hasArg {
		return c.arg
	}
	return 1
}

func (le *LineEditor) viListOrEof(c cmdCtx) {
	if le.bufferEmpty() {
		le.eof = true
		le.finish()
	} else {
		le.edNewline(c)
	}
}

func (le *LineEditor) edDeleteNextChar(c cmdCtx) {
	arg := c.arg
	for {
		byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
		if le.CurrentLine() != "" && byteSize != 0 {
			line, mbchar := byteslicePop(le.CurrentLine(), le.bytePointer, byteSize)
			le.copyForVi(mbchar)
			if le.bytePointer > 0 && len(le.CurrentLine()) == le.bytePointer+byteSize {
				prev := getPrevMbcharSize(line, le.bytePointer)
				le.setCurrentLine(line, le.bytePointer-prev, true)
			} else {
				le.setCurrentLine(line, le.bytePointer, true)
			}
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viToHistoryLine(cmdCtx) {
	if le.hist.Empty() {
		return
	}
	le.moveHistory(0, hasPtr(true), lineStart, cursorStart, 0, 0)
}

func (le *LineEditor) viPastePrev(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.viClipboard) > 0 {
			gcs := graphemeClusters(le.viClipboard)
			cursorPoint := ""
			if len(gcs) > 1 {
				cursorPoint = strings.Join(gcs[:len(gcs)-1], "")
			}
			le.setCurrentLine(byteinsert(le.CurrentLine(), le.bytePointer, le.viClipboard), le.bytePointer+len(cursorPoint), true)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) viPasteNext(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.viClipboard) > 0 {
			byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
			line := byteinsert(le.CurrentLine(), le.bytePointer+byteSize, le.viClipboard)
			le.setCurrentLine(line, le.bytePointer+len(le.viClipboard), true)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) edArgumentDigit(c cmdCtx) {
	num := firstDigit(c.key)
	cur := 0
	if le.hasViArg {
		cur = le.viArg
	}
	le.viArg = cur*10 + num
	le.hasViArg = true
}

func firstDigit(key string) int {
	for _, r := range key {
		if r >= '0' && r <= '9' {
			return int(r - '0')
		}
	}
	return 0
}

func (le *LineEditor) viToColumn(c cmdCtx) {
	totalByteSize := 0
	totalWidth := 0
	for _, gc := range graphemeClusters(le.CurrentLine()) {
		w := getMbcharWidth(gc)
		if totalWidth+w >= c.arg {
			break
		}
		totalByteSize += len(gc)
		totalWidth += w
	}
	le.bytePointer = totalByteSize
}

func (le *LineEditor) viReplaceChar(c cmdCtx) {
	arg := c.arg
	le.waitingProc = func(k, _ string) {
		if arg == 1 {
			byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
			before := byteslice(le.CurrentLine(), 0, le.bytePointer)
			remaining := le.bytePointer + byteSize
			after := le.CurrentLine()[remaining:]
			le.setCurrentLine(before+k+after, 0, false)
			le.waitingProc = nil
		} else if arg > 1 {
			byteSize := 0
			for i := 0; i < arg; i++ {
				byteSize += getNextMbcharSize(le.CurrentLine(), le.bytePointer+byteSize)
			}
			before := byteslice(le.CurrentLine(), 0, le.bytePointer)
			remaining := le.bytePointer + byteSize
			after := le.CurrentLine()[remaining:]
			replaced := strings.Repeat(k, arg)
			le.setCurrentLine(before+replaced+after, le.bytePointer+len(replaced), true)
			le.waitingProc = nil
		}
	}
}

func (le *LineEditor) viNextChar(c cmdCtx) {
	arg := c.arg
	inclusive := c.inclusive
	le.waitingProc = func(k, _ string) { le.searchNextChar(k, arg, false, inclusive) }
}
func (le *LineEditor) viToNextChar(c cmdCtx) {
	arg := c.arg
	inclusive := c.inclusive
	le.waitingProc = func(k, _ string) { le.searchNextChar(k, arg, true, inclusive) }
}

func (le *LineEditor) searchNextChar(key string, arg int, needPrevChar, inclusive bool) {
	var prevTotal, total *[2]int
	found := false
	for _, mbchar := range graphemeClusters(le.CurrentLine()[le.bytePointer:]) {
		if total == nil {
			w := getMbcharWidth(mbchar)
			total = &[2]int{len(mbchar), w}
		} else {
			if key == mbchar {
				arg--
				if arg == 0 {
					found = true
					break
				}
			}
			w := getMbcharWidth(mbchar)
			pt := *total
			prevTotal = &pt
			total = &[2]int{total[0] + len(mbchar), total[1] + w}
		}
	}
	if !needPrevChar && found && total != nil {
		le.bytePointer += total[0]
	} else if needPrevChar && found && prevTotal != nil {
		le.bytePointer += prevTotal[0]
	}
	if inclusive {
		byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
		if byteSize > 0 {
			le.bytePointer += byteSize
		}
	}
	le.waitingProc = nil
}

func (le *LineEditor) viPrevChar(c cmdCtx) {
	arg := c.arg
	le.waitingProc = func(k, _ string) { le.searchPrevChar(k, arg, false) }
}
func (le *LineEditor) viToPrevChar(c cmdCtx) {
	arg := c.arg
	le.waitingProc = func(k, _ string) { le.searchPrevChar(k, arg, true) }
}

func (le *LineEditor) searchPrevChar(key string, arg int, needNextChar bool) {
	var prevTotal, total *[2]int
	found := false
	gcs := graphemeClusters(byteslice(le.CurrentLine(), 0, le.bytePointer+1))
	for i := len(gcs) - 1; i >= 0; i-- {
		mbchar := gcs[i]
		if total == nil {
			w := getMbcharWidth(mbchar)
			total = &[2]int{len(mbchar), w}
		} else {
			if key == mbchar {
				arg--
				if arg == 0 {
					found = true
					break
				}
			}
			w := getMbcharWidth(mbchar)
			pt := *total
			prevTotal = &pt
			total = &[2]int{total[0] + len(mbchar), total[1] + w}
		}
	}
	if !needNextChar && found && total != nil {
		le.bytePointer -= total[0]
	} else if needNextChar && found && prevTotal != nil {
		le.bytePointer -= prevTotal[0]
	}
	le.waitingProc = nil
}

func (le *LineEditor) viJoinLines(c cmdCtx) {
	arg := c.arg
	for {
		if len(le.bufferOfLines) > le.lineIndex+1 {
			nextLine := strings.TrimLeft(le.bufferOfLines[le.lineIndex+1], " \t\n\r\f\v")
			le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex+1], le.bufferOfLines[le.lineIndex+2:]...)
			le.setCurrentLine(le.CurrentLine()+" "+nextLine, len(le.CurrentLine()), true)
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) undo(cmdCtx)    { le.moveUndoRedo(-1) }
func (le *LineEditor) redoCmd(cmdCtx) { le.moveUndoRedo(+1) }

func (le *LineEditor) moveUndoRedo(direction int) {
	le.restoring = true
	idx := le.undoRedoIndex + direction
	if idx < 0 || idx > len(le.undoRedoHistory)-1 {
		return
	}
	le.undoRedoIndex = idx
	e := le.undoRedoHistory[idx]
	le.bufferOfLines = append([]string(nil), e.buffer...)
	le.lineIndex = e.lineIndex
	le.bytePointer = e.bytePtr
}
