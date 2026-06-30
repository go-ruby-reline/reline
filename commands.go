package reline

import "strings"

// argumentableMethods lists commands that honor a numeric vi argument (the
// methods declared with `arg:` in MRI). inclusiveMethods lists motions that take
// `inclusive:` (consume the char under the cursor when used with an operator).
var argumentableMethods = map[string]bool{
	"ed_next_char": true, "ed_prev_char": true, "ed_prev_history": true,
	"ed_next_history": true, "ed_search_prev_history": true, "ed_search_next_history": true,
	"ed_delete_prev_char": true, "ed_delete_next_char": true,
	"vi_next_word": true, "vi_prev_word": true, "vi_end_word": true,
	"vi_next_big_word": true, "vi_prev_big_word": true, "vi_end_big_word": true,
	"vi_to_column": true, "vi_replace_char": true, "vi_next_char": true,
	"vi_to_next_char": true, "vi_prev_char": true, "vi_to_prev_char": true,
	"vi_paste_prev": true, "vi_paste_next": true, "vi_join_lines": true,
	"vi_change_meta": true, "vi_delete_meta": true, "vi_yank": true,
	"em_delete_prev_char": true,
}

var inclusiveMethods = map[string]bool{
	"vi_end_word": true, "vi_end_big_word": true,
	"vi_next_char": true, "vi_to_next_char": true,
}

// wrapMethodCall invokes the command named methodSymbol, threading a waiting
// proc (char-search/replace), the numeric argument, and the operator's
// inclusive flag (MRI wrap_method_call).
func (le *LineEditor) wrapMethodCall(methodSymbol, key string, withOperator bool) {
	if le.waitingProc != nil {
		le.waitingProc(key, methodSymbol)
		return
	}
	fn, ok := commandTable[methodSymbol]
	if !ok {
		return
	}
	arg := 1
	if le.hasViArg {
		arg = le.viArg
	}
	ctx := cmdCtx{key: key, arg: arg, hasArg: le.hasViArg && argumentableMethods[methodSymbol], inclusive: inclusiveMethods[methodSymbol] && withOperator}
	fn(le, ctx)
}

// cmdCtx carries per-invocation command parameters.
type cmdCtx struct {
	key       string
	arg       int
	hasArg    bool
	inclusive bool
}

// commandFn is a LineEditor command implementation.
type commandFn func(*LineEditor, cmdCtx)

// dispatchOperator runs a pending vi operator (change/delete/yank) over the
// motion's byte delta (MRI __send__(@vi_waiting_operator, diff)).
func (le *LineEditor) dispatchOperator(op string, diff int) {
	switch op {
	case "vi_change_meta_confirm":
		le.viChangeMetaConfirm(diff)
	case "vi_delete_meta_confirm":
		le.viDeleteMetaConfirm(diff)
	case "vi_yank_confirm":
		le.viYankConfirm(diff)
	}
}

// commandTable maps each MRI command symbol (and its aliases) to its Go impl.
var commandTable map[string]commandFn

func init() {
	commandTable = map[string]commandFn{
		"ed_insert":               (*LineEditor).edInsert,
		"self_insert":             (*LineEditor).edInsert,
		"ed_digit":                (*LineEditor).edDigit,
		"insert_raw_char":         (*LineEditor).insertRawChar,
		"ed_ignore":               (*LineEditor).edIgnore,
		"ed_unassigned":           (*LineEditor).edIgnore,
		"ed_next_char":            (*LineEditor).edNextChar,
		"forward_char":            (*LineEditor).edNextChar,
		"ed_prev_char":            (*LineEditor).edPrevChar,
		"backward_char":           (*LineEditor).edPrevChar,
		"ed_move_to_beg":          (*LineEditor).edMoveToBeg,
		"beginning_of_line":       (*LineEditor).edMoveToBeg,
		"ed_move_to_end":          (*LineEditor).edMoveToEnd,
		"end_of_line":             (*LineEditor).edMoveToEnd,
		"vi_first_print":          (*LineEditor).viFirstPrint,
		"vi_zero":                 (*LineEditor).viZero,
		"ed_newline":              (*LineEditor).edNewline,
		"ed_force_submit":         (*LineEditor).edForceSubmit,
		"em_delete_prev_char":     (*LineEditor).emDeletePrevChar,
		"backward_delete_char":    (*LineEditor).emDeletePrevChar,
		"ed_kill_line":            (*LineEditor).edKillLine,
		"kill_line":               (*LineEditor).edKillLine,
		"vi_change_to_eol":        (*LineEditor).viChangeToEol,
		"vi_kill_line_prev":       (*LineEditor).viKillLinePrev,
		"unix_line_discard":       (*LineEditor).viKillLinePrev,
		"em_kill_line":            (*LineEditor).emKillLine,
		"kill_whole_line":         (*LineEditor).emKillLine,
		"em_delete":               (*LineEditor).emDelete,
		"delete_char":             (*LineEditor).emDelete,
		"em_delete_or_list":       (*LineEditor).emDeleteOrList,
		"delete_char_or_list":     (*LineEditor).emDeleteOrList,
		"em_yank":                 (*LineEditor).emYank,
		"yank":                    (*LineEditor).emYank,
		"em_yank_pop":             (*LineEditor).emYankPop,
		"yank_pop":                (*LineEditor).emYankPop,
		"ed_clear_screen":         (*LineEditor).edClearScreen,
		"clear_screen":            (*LineEditor).edClearScreen,
		"em_next_word":            (*LineEditor).emNextWord,
		"forward_word":            (*LineEditor).emNextWord,
		"ed_prev_word":            (*LineEditor).edPrevWord,
		"backward_word":           (*LineEditor).edPrevWord,
		"em_delete_next_word":     (*LineEditor).emDeleteNextWord,
		"kill_word":               (*LineEditor).emDeleteNextWord,
		"ed_delete_prev_word":     (*LineEditor).edDeletePrevWord,
		"backward_kill_word":      (*LineEditor).edDeletePrevWord,
		"ed_transpose_chars":      (*LineEditor).edTransposeChars,
		"transpose_chars":         (*LineEditor).edTransposeChars,
		"ed_transpose_words":      (*LineEditor).edTransposeWords,
		"transpose_words":         (*LineEditor).edTransposeWords,
		"em_capitol_case":         (*LineEditor).emCapitolCase,
		"capitalize_word":         (*LineEditor).emCapitolCase,
		"em_lower_case":           (*LineEditor).emLowerCase,
		"downcase_word":           (*LineEditor).emLowerCase,
		"em_upper_case":           (*LineEditor).emUpperCase,
		"upcase_word":             (*LineEditor).emUpperCase,
		"em_kill_region":          (*LineEditor).emKillRegion,
		"unix_word_rubout":        (*LineEditor).emKillRegion,
		"em_set_mark":             (*LineEditor).emSetMark,
		"set_mark":                (*LineEditor).emSetMark,
		"em_exchange_mark":        (*LineEditor).emExchangeMark,
		"exchange_point_and_mark": (*LineEditor).emExchangeMark,
		"ed_prev_history":         (*LineEditor).edPrevHistory,
		"previous_history":        (*LineEditor).edPrevHistory,
		"ed_next_history":         (*LineEditor).edNextHistory,
		"next_history":            (*LineEditor).edNextHistory,
		"ed_beginning_of_history": (*LineEditor).edBeginningOfHistory,
		"beginning_of_history":    (*LineEditor).edBeginningOfHistory,
		"ed_end_of_history":       (*LineEditor).edEndOfHistory,
		"end_of_history":          (*LineEditor).edEndOfHistory,
		"ed_search_prev_history":  (*LineEditor).edSearchPrevHistory,
		"history_search_backward": (*LineEditor).edSearchPrevHistory,
		"ed_search_next_history":  (*LineEditor).edSearchNextHistory,
		"history_search_forward":  (*LineEditor).edSearchNextHistory,
		"vi_search_prev":          (*LineEditor).viSearchPrev,
		"reverse_search_history":  (*LineEditor).viSearchPrev,
		"vi_search_next":          (*LineEditor).viSearchNext,
		"forward_search_history":  (*LineEditor).viSearchNext,
		"complete":                (*LineEditor).complete,
		"menu_complete":           (*LineEditor).menuComplete,
		"menu_complete_backward":  (*LineEditor).menuCompleteBackward,
		"completion_journey_up":   (*LineEditor).completionJourneyUp,
		"vi_insert":               (*LineEditor).viInsert,
		"vi_add":                  (*LineEditor).viAdd,
		"vi_command_mode":         (*LineEditor).viCommandMode,
		"vi_movement_mode":        (*LineEditor).viCommandMode,
		"vi_next_word":            (*LineEditor).viNextWord,
		"vi_prev_word":            (*LineEditor).viPrevWord,
		"vi_end_word":             (*LineEditor).viEndWord,
		"vi_next_big_word":        (*LineEditor).viNextBigWord,
		"vi_prev_big_word":        (*LineEditor).viPrevBigWord,
		"vi_end_big_word":         (*LineEditor).viEndBigWord,
		"vi_delete_prev_char":     (*LineEditor).viDeletePrevChar,
		"vi_insert_at_bol":        (*LineEditor).viInsertAtBol,
		"vi_add_at_eol":           (*LineEditor).viAddAtEol,
		"ed_delete_prev_char":     (*LineEditor).edDeletePrevChar,
		"ed_delete_next_char":     (*LineEditor).edDeleteNextChar,
		"vi_change_meta":          (*LineEditor).viChangeMeta,
		"vi_delete_meta":          (*LineEditor).viDeleteMeta,
		"vi_yank":                 (*LineEditor).viYank,
		"vi_list_or_eof":          (*LineEditor).viListOrEof,
		"vi_end_of_transmission":  (*LineEditor).viListOrEof,
		"vi_eof_maybe":            (*LineEditor).viListOrEof,
		"vi_to_history_line":      (*LineEditor).viToHistoryLine,
		"vi_paste_prev":           (*LineEditor).viPastePrev,
		"vi_paste_next":           (*LineEditor).viPasteNext,
		"ed_argument_digit":       (*LineEditor).edArgumentDigit,
		"vi_to_column":            (*LineEditor).viToColumn,
		"vi_replace_char":         (*LineEditor).viReplaceChar,
		"vi_next_char":            (*LineEditor).viNextChar,
		"vi_to_next_char":         (*LineEditor).viToNextChar,
		"vi_prev_char":            (*LineEditor).viPrevChar,
		"vi_to_prev_char":         (*LineEditor).viToPrevChar,
		"vi_join_lines":           (*LineEditor).viJoinLines,
		"emacs_editing_mode":      (*LineEditor).emacsEditingMode,
		"vi_editing_mode":         (*LineEditor).viEditingMode,
		"undo":                    (*LineEditor).undo,
		"redo":                    (*LineEditor).redoCmd,
		"key_newline":             (*LineEditor).keyNewlineCmd,
	}
}

// edIgnore is the no-op command (MRI ed_ignore): bound to keys like Ctrl-C /
// Ctrl-Z that should be swallowed without editing effect.
func (le *LineEditor) edIgnore(cmdCtx) {
	_ = le
}

// ---- insertion ----

func (le *LineEditor) edInsert(c cmdCtx) {
	str := c.key
	if le.inPasting {
		le.continuousInsertBuffer += str
		return
	} else if le.continuousInsertBuffer != "" {
		le.processInsert(false)
	}
	le.InsertText(str)
}

func (le *LineEditor) edDigit(c cmdCtx) {
	if le.hasViArg {
		le.edArgumentDigit(c)
	} else {
		le.edInsert(c)
	}
}

func (le *LineEditor) insertRawChar(c cmdCtx) {
	for i := 0; i < c.arg; i++ {
		if c.key == "\n" || c.key == "\r" {
			le.keyNewline()
		} else if c.key != "\x00" {
			le.edInsert(cmdCtx{key: c.key})
		}
	}
}

// ---- cursor movement ----

func (le *LineEditor) edNextChar(c cmdCtx) {
	arg := c.arg
	for {
		byteSize := getNextMbcharSize(le.CurrentLine(), le.bytePointer)
		if le.bytePointer < len(le.CurrentLine()) {
			le.bytePointer += byteSize
		} else if le.config.EditingModeIs(ModeEmacs) && le.bytePointer == len(le.CurrentLine()) && le.lineIndex < len(le.bufferOfLines)-1 {
			le.bytePointer = 0
			le.lineIndex++
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) edPrevChar(c cmdCtx) {
	arg := c.arg
	for {
		if le.bytePointer > 0 {
			byteSize := getPrevMbcharSize(le.CurrentLine(), le.bytePointer)
			le.bytePointer -= byteSize
		} else if le.config.EditingModeIs(ModeEmacs) && le.bytePointer == 0 && le.lineIndex > 0 {
			le.lineIndex--
			le.bytePointer = len(le.CurrentLine())
		}
		arg--
		if arg <= 0 {
			break
		}
	}
}

func (le *LineEditor) edMoveToBeg(cmdCtx) { le.bytePointer = 0 }
func (le *LineEditor) edMoveToEnd(cmdCtx) { le.bytePointer = len(le.CurrentLine()) }
func (le *LineEditor) viFirstPrint(cmdCtx) {
	le.bytePointer = viFirstPrint(le.CurrentLine())
}
func (le *LineEditor) viZero(c cmdCtx) {
	if le.hasViArg {
		le.edArgumentDigit(c)
	} else {
		le.edMoveToBeg(c)
	}
}

// ---- newline / submit ----

func (le *LineEditor) keyNewline() {
	if le.isMultiline {
		nextLine := le.CurrentLine()[le.bytePointer:]
		cursorLine := byteslice(le.CurrentLine(), 0, le.bytePointer)
		le.insertNewLine(cursorLine, nextLine)
	}
}

func (le *LineEditor) keyNewlineCmd(cmdCtx) { le.keyNewline() }

func (le *LineEditor) insertNewLine(cursorLine, nextLine string) {
	tail := append([]string{nextLine}, le.bufferOfLines[le.lineIndex+1:]...)
	le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex+1], tail...)
	le.bufferOfLines[le.lineIndex] = cursorLine
	le.lineIndex++
	le.bytePointer = 0
}

func (le *LineEditor) edNewline(cmdCtx) {
	le.processInsert(true)
	if le.isMultiline {
		if le.config.EditingModeIs(ModeViCommand) {
			if le.lineIndex < len(le.bufferOfLines)-1 {
				le.edNextHistory(cmdCtx{arg: 1})
			} else {
				le.finish()
			}
		} else {
			if le.lineIndex == len(le.bufferOfLines)-1 && le.confirmMultilineTerminationCall() {
				le.finish()
			} else {
				le.keyNewline()
			}
		}
	} else {
		le.finish()
	}
}

func (le *LineEditor) edForceSubmit(cmdCtx) {
	le.processInsert(true)
	le.finish()
}

// ---- deletion / kill ----

func (le *LineEditor) emDeletePrevChar(c cmdCtx) {
	arg := c.arg
	for i := 0; i < arg; i++ {
		if le.bytePointer == 0 && le.lineIndex > 0 {
			le.bytePointer = len(le.bufferOfLines[le.lineIndex-1])
			le.bufferOfLines[le.lineIndex-1] += le.bufferOfLines[le.lineIndex]
			le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex], le.bufferOfLines[le.lineIndex+1:]...)
			le.lineIndex--
		} else if le.bytePointer > 0 {
			byteSize := getPrevMbcharSize(le.CurrentLine(), le.bytePointer)
			line, _ := byteslicePop(le.CurrentLine(), le.bytePointer-byteSize, byteSize)
			le.setCurrentLine(line, le.bytePointer-byteSize, true)
		}
	}
}

func (le *LineEditor) edKillLine(cmdCtx) {
	cur := le.CurrentLine()
	if len(cur) > le.bytePointer {
		line, deleted := byteslicePop(cur, le.bytePointer, len(cur)-le.bytePointer)
		le.setCurrentLine(line, len(line), true)
		le.killRing.Append(deleted, false)
	} else if le.bytePointer == len(cur) && len(le.bufferOfLines) > le.lineIndex+1 {
		next := le.bufferOfLines[le.lineIndex+1]
		le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex+1], le.bufferOfLines[le.lineIndex+2:]...)
		le.setCurrentLine(cur+next, len(cur), true)
	}
}

func (le *LineEditor) viChangeToEol(c cmdCtx) {
	le.edKillLine(c)
	le.config.SetEditingMode(ModeViInsert)
}

func (le *LineEditor) viKillLinePrev(cmdCtx) {
	if le.bytePointer > 0 {
		line, deleted := byteslicePop(le.CurrentLine(), 0, le.bytePointer)
		le.setCurrentLine(line, 0, true)
		le.killRing.Append(deleted, true)
	}
}

func (le *LineEditor) emKillLine(cmdCtx) {
	if len(le.CurrentLine()) > 0 {
		le.killRing.Append(le.CurrentLine(), true)
		le.setCurrentLine("", 0, true)
	}
}

func (le *LineEditor) emDelete(c cmdCtx) {
	cur := le.CurrentLine()
	if le.bufferEmpty() && c.key == "\x04" {
		le.eof = true
		le.finish()
	} else if le.bytePointer < len(cur) {
		splittedLast := cur[le.bytePointer:]
		gcs := graphemeClusters(splittedLast)
		mbchar := gcs[0]
		line, _ := byteslicePop(cur, le.bytePointer, len(mbchar))
		le.setCurrentLine(line, 0, false)
	} else if le.bytePointer == len(cur) && len(le.bufferOfLines) > le.lineIndex+1 {
		next := le.bufferOfLines[le.lineIndex+1]
		le.bufferOfLines = append(le.bufferOfLines[:le.lineIndex+1], le.bufferOfLines[le.lineIndex+2:]...)
		le.setCurrentLine(cur+next, len(cur), true)
	}
}

func (le *LineEditor) emDeleteOrList(c cmdCtx) {
	if le.CurrentLine() == "" || le.bytePointer < len(le.CurrentLine()) {
		le.emDelete(c)
	} else if !le.config.Autocompletion {
		pre, target, post, _ := le.retrieveCompletionBlock()
		result := le.callCompletionProc(pre, target, post)
		if result != nil {
			candidates := le.filterNormalizeCandidates(target, result)
			le.menu(candidates)
		}
	}
}

func (le *LineEditor) emYank(cmdCtx) {
	if yanked, ok := le.killRing.Yank(); ok {
		le.InsertText(yanked)
	}
}

func (le *LineEditor) emYankPop(cmdCtx) {
	yanked, prevYank, ok := le.killRing.YankPop()
	if ok {
		line, _ := byteslicePop(le.CurrentLine(), le.bytePointer-len(prevYank), len(prevYank))
		le.setCurrentLine(line, le.bytePointer-len(prevYank), true)
		le.InsertText(yanked)
	}
}

func (le *LineEditor) edClearScreen(cmdCtx) {
	le.io.ClearScreen()
	le.screenRows, le.screenCols = le.io.GetScreenSize()
}

// ---- word movement / mutation ----

func (le *LineEditor) emNextWord(cmdCtx) {
	if len(le.CurrentLine()) > le.bytePointer {
		le.bytePointer += emForwardWord(le.CurrentLine(), le.bytePointer)
	}
}

func (le *LineEditor) edPrevWord(cmdCtx) {
	if le.bytePointer > 0 {
		le.bytePointer -= emBackwardWord(le.CurrentLine(), le.bytePointer)
	}
}

func (le *LineEditor) emDeleteNextWord(cmdCtx) {
	if len(le.CurrentLine()) > le.bytePointer {
		byteSize := emForwardWord(le.CurrentLine(), le.bytePointer)
		line, word := byteslicePop(le.CurrentLine(), le.bytePointer, byteSize)
		le.setCurrentLine(line, 0, false)
		le.killRing.Append(word, false)
	}
}

func (le *LineEditor) edDeletePrevWord(cmdCtx) {
	if le.bytePointer > 0 {
		byteSize := emBackwardWord(le.CurrentLine(), le.bytePointer)
		line, word := byteslicePop(le.CurrentLine(), le.bytePointer-byteSize, byteSize)
		le.setCurrentLine(line, le.bytePointer-byteSize, true)
		le.killRing.Append(word, true)
	}
}

func (le *LineEditor) edTransposeChars(cmdCtx) {
	if le.bytePointer > 0 {
		if le.bytePointer < len(le.CurrentLine()) {
			le.bytePointer += getNextMbcharSize(le.CurrentLine(), le.bytePointer)
		}
		back1 := getPrevMbcharSize(le.CurrentLine(), le.bytePointer)
		if le.bytePointer-back1 > 0 {
			back2 := getPrevMbcharSize(le.CurrentLine(), le.bytePointer-back1)
			back2Pointer := le.bytePointer - back1 - back2
			line, back2Mbchar := byteslicePop(le.CurrentLine(), back2Pointer, back2)
			le.setCurrentLine(byteinsert(line, le.bytePointer-back2, back2Mbchar), 0, false)
		}
	}
}

func (le *LineEditor) edTransposeWords(cmdCtx) {
	cur := le.CurrentLine()
	lws, ms, rws, as := edTransposeWords(cur, le.bytePointer)
	before := byteslice(cur, 0, lws)
	leftWord := byteslice(cur, lws, ms-lws)
	middle := byteslice(cur, ms, rws-ms)
	rightWord := byteslice(cur, rws, as-rws)
	after := byteslice(cur, as, len(cur)-as)
	if leftWord == "" || rightWord == "" {
		return
	}
	head := before + rightWord + middle + leftWord
	le.setCurrentLine(head+after, len(head), true)
}

func (le *LineEditor) emCapitolCase(cmdCtx) {
	if len(le.CurrentLine()) > le.bytePointer {
		byteSize, newStr := emForwardWordWithCapitalization(le.CurrentLine(), le.bytePointer)
		before := byteslice(le.CurrentLine(), 0, le.bytePointer)
		after := le.CurrentLine()[le.bytePointer+byteSize:]
		le.setCurrentLine(before+newStr+after, le.bytePointer+len(newStr), true)
	}
}

func (le *LineEditor) emLowerCase(cmdCtx) {
	le.caseWord(strings.ToLower, true)
}
func (le *LineEditor) emUpperCase(cmdCtx) {
	le.caseWord(strings.ToUpper, false)
}

func (le *LineEditor) caseWord(conv func(string) string, lower bool) {
	if len(le.CurrentLine()) > le.bytePointer {
		byteSize := emForwardWord(le.CurrentLine(), le.bytePointer)
		seg := byteslice(le.CurrentLine(), le.bytePointer, byteSize)
		var sb strings.Builder
		for _, gc := range graphemeClusters(seg) {
			if lower {
				if isUpperGC(gc) {
					sb.WriteString(strings.ToLower(gc))
					continue
				}
			} else {
				if isLowerGC(gc) {
					sb.WriteString(strings.ToUpper(gc))
					continue
				}
			}
			sb.WriteString(gc)
		}
		rest := le.CurrentLine()[le.bytePointer+byteSize:]
		line := byteslice(le.CurrentLine(), 0, le.bytePointer) + sb.String()
		le.setCurrentLine(line+rest, len(line), true)
	}
}

func isUpperGC(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}
func isLowerGC(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

func (le *LineEditor) emKillRegion(cmdCtx) {
	if le.bytePointer > 0 {
		byteSize := emBigBackwardWord(le.CurrentLine(), le.bytePointer)
		line, deleted := byteslicePop(le.CurrentLine(), le.bytePointer-byteSize, byteSize)
		le.setCurrentLine(line, le.bytePointer-byteSize, true)
		le.killRing.Append(deleted, true)
	}
}

func (le *LineEditor) emSetMark(cmdCtx) {
	le.markPointer = [2]int{le.bytePointer, le.lineIndex}
	le.hasMark = true
}

func (le *LineEditor) emExchangeMark(cmdCtx) {
	if !le.hasMark {
		return
	}
	newPointer := [2]int{le.bytePointer, le.lineIndex}
	le.bytePointer, le.lineIndex = le.markPointer[0], le.markPointer[1]
	le.markPointer = newPointer
}

// ---- mode switches ----

func (le *LineEditor) viInsert(cmdCtx) { le.config.SetEditingMode(ModeViInsert) }
func (le *LineEditor) viAdd(c cmdCtx) {
	le.config.SetEditingMode(ModeViInsert)
	le.edNextChar(cmdCtx{arg: 1})
}
func (le *LineEditor) viCommandMode(c cmdCtx) {
	le.edPrevChar(cmdCtx{arg: 1})
	le.config.SetEditingMode(ModeViCommand)
}
func (le *LineEditor) viInsertAtBol(c cmdCtx) {
	le.edMoveToBeg(c)
	le.config.SetEditingMode(ModeViInsert)
}
func (le *LineEditor) viAddAtEol(c cmdCtx)     { le.edMoveToEnd(c); le.config.SetEditingMode(ModeViInsert) }
func (le *LineEditor) emacsEditingMode(cmdCtx) { le.config.SetEditingMode(ModeEmacs) }
func (le *LineEditor) viEditingMode(cmdCtx)    { le.config.SetEditingMode(ModeViInsert) }
