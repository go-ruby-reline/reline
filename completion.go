package reline

import "strings"

// CompleterWordBreakCharacters and CompleterQuoteCharacters mirror Reline's
// defaults; they delimit the word under the cursor for completion.
var (
	CompleterWordBreakCharacters = " \t\n`><=;|&{("
	CompleterQuoteCharacters     = "\"'"
)

// retrieveCompletionBlock splits the line at the cursor into (preposing, target,
// postposing, quote): the word being completed plus surrounding context, with
// quote detection (MRI retrieve_completion_block).
func (le *LineEditor) retrieveCompletionBlock() (string, string, string, string) {
	quoteChars := CompleterQuoteCharacters
	before := graphemeClusters(byteslice(le.CurrentLine(), 0, le.bytePointer))
	quote := ""
	if len(le.CurrentLine()) == le.bytePointer && quoteChars != "" {
		escaped := false
		for _, c := range before {
			if escaped {
				escaped = false
				continue
			} else if c == "\\" {
				escaped = true
			} else if quote != "" {
				if c == quote {
					quote = ""
				}
			} else if strings.Contains(quoteChars, c) && len(c) == 1 {
				quote = c
			}
		}
	}
	wordBreak := quoteChars + CompleterWordBreakCharacters
	breakIndex := -1
	for i := len(before) - 1; i >= 0; i-- {
		c := before[i]
		if (len(c) == 1 && strings.Contains(wordBreak, c)) || (len(c) == 1 && strings.Contains(quoteChars, c)) {
			breakIndex = i
			break
		}
	}
	preposing := strings.Join(before[:breakIndex+1], "")
	target := strings.Join(before[breakIndex+1:], "")
	postposing := le.CurrentLine()[le.bytePointer:]
	lines := le.WholeLines()
	if le.lineIndex > 0 {
		preposing = strings.Join(lines[:le.lineIndex], "\n") + "\n" + preposing
	}
	if len(lines)-1 > le.lineIndex {
		postposing = postposing + "\n" + strings.Join(lines[le.lineIndex+1:], "\n")
	}
	return preposing, target, postposing, quote
}

func (le *LineEditor) callCompletionProc(pre, target, post string) []string {
	if le.completionProc != nil && target != "" {
		return le.completionProc(target, pre, post)
	}
	return nil
}

func (le *LineEditor) filterNormalizeCandidates(target string, list []string) []string {
	t := target
	if le.completionIgnoreCase {
		t = strings.ToLower(t)
	}
	seen := map[string]bool{}
	var out []string
	for _, item := range list {
		if item == "" {
			continue
		}
		var match bool
		if le.completionIgnoreCase {
			match = strings.HasPrefix(strings.ToLower(item), t)
		} else {
			match = strings.HasPrefix(item, target)
		}
		if match && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func (le *LineEditor) menu(list []string) {
	le.menuInfo = list
	le.hasMenu = true
}

// MenuInfo returns the pending completion menu candidates (and whether a menu is
// active). Rendering of the menu is the embedder's concern; the state lives here.
func (le *LineEditor) MenuInfo() ([]string, bool) {
	return le.menuInfo, le.hasMenu
}

// performCompletion drives the Tab completion state machine: it computes the
// common prefix, may insert it, and transitions between PERFECT_MATCH / MENU
// states (MRI perform_completion).
func (le *LineEditor) performCompletion(preposing, target, postposing, quote string, list []string) {
	candidates := le.filterNormalizeCandidates(target, list)

	switch le.completionState {
	case csPerfectMatch:
		// no dig_perfect_match_proc seam in the pure core
	case csMenu:
		le.menu(candidates)
		return
	case csMenuWithPerfectMatch:
		le.menu(candidates)
		le.completionState = csPerfectMatch
		return
	}

	completed := CommonPrefix(candidates, le.completionIgnoreCase)
	if completed == "" {
		return
	}

	appendChar := ""
	if contains(candidates, completed) {
		if len(candidates) == 1 {
			if quote != "" {
				appendChar = quote
			} else {
				appendChar = le.completionAppendChar
			}
			le.completionState = csPerfectMatch
		} else if le.config.ShowAllIfAmbiguous {
			le.menu(candidates)
			le.completionState = csPerfectMatch
		} else {
			le.completionState = csMenuWithPerfectMatch
		}
		le.perfectMatched = completed
	} else {
		le.completionState = csMenu
		if le.config.ShowAllIfAmbiguous {
			le.menu(candidates)
		}
	}
	whole := preposing + completed + appendChar + postposing
	le.bufferOfLines[le.lineIndex] = nthLine(strings.Split(whole, "\n"), le.lineIndex)
	lineToPointer := nthLine(strings.Split(preposing+completed+appendChar, "\n"), le.lineIndex)
	le.bytePointer = len(lineToPointer)
}

func nthLine(lines []string, n int) string {
	if n < len(lines) {
		return lines[n]
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func (le *LineEditor) complete(cmdCtx) {
	if le.config.DisableCompletion {
		return
	}
	le.processInsert(true)
	if le.config.Autocompletion {
		le.completionState = csNormal
		le.completionOccurs = le.moveCompletedList(journeyDown)
	} else {
		le.completionJourney = nil
		pre, target, post, quote := le.retrieveCompletionBlock()
		result := le.callCompletionProc(pre, target, post)
		if result != nil {
			le.completionOccurs = true
			le.performCompletion(pre, target, post, quote, result)
		}
	}
}

type journeyDir int

const (
	journeyUp journeyDir = iota
	journeyDown
)

func (le *LineEditor) completionJourneyMove(direction journeyDir) {
	if le.config.DisableCompletion {
		return
	}
	le.processInsert(true)
	le.completionState = csNormal
	le.completionOccurs = le.moveCompletedList(direction)
}

func (le *LineEditor) menuComplete(cmdCtx)         { le.completionJourneyMove(journeyDown) }
func (le *LineEditor) menuCompleteBackward(cmdCtx) { le.completionJourneyMove(journeyUp) }
func (le *LineEditor) completionJourneyUp(cmdCtx) {
	if le.config.Autocompletion {
		le.completionJourneyMove(journeyUp)
	}
}

// moveCompletedList cycles through the completion candidates in place
// (MRI move_completed_list), used by autocompletion / menu-complete.
func (le *LineEditor) moveCompletedList(direction journeyDir) bool {
	if le.completionJourney == nil {
		le.completionJourney = le.retrieveCompletionJourneyState()
	}
	if le.completionJourney == nil {
		return false
	}
	js := le.completionJourney
	delta := -1
	if direction == journeyDown {
		delta = 1
	}
	js.pointer = ((js.pointer+delta)%len(js.list) + len(js.list)) % len(js.list)
	completed := js.list[js.pointer]
	le.setCurrentLine(js.pre+completed+js.post, len(js.pre)+len(completed), true)
	return true
}

func (le *LineEditor) retrieveCompletionJourneyState() *completionJourneyState {
	preposing, target, postposing, _ := le.retrieveCompletionBlock()
	list := le.callCompletionProc(preposing, target, postposing)
	if list == nil {
		return nil
	}
	var candidates []string
	for _, item := range list {
		if strings.HasPrefix(item, target) {
			candidates = append(candidates, item)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	preParts := strings.Split(preposing, "\n")
	pre := ""
	if len(preParts) > 0 {
		pre = preParts[len(preParts)-1]
	}
	postParts := strings.Split(postposing, "\n")
	post := ""
	if len(postParts) > 0 {
		post = postParts[0]
	}
	return &completionJourneyState{
		lineIndex: le.lineIndex,
		pre:       pre,
		target:    target,
		post:      post,
		list:      append([]string{target}, candidates...),
		pointer:   0,
	}
}
