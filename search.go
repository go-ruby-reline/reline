package reline

import (
	"fmt"
	"strings"
)

// searchDirection selects reverse (Ctrl-R) or forward (Ctrl-S) incremental
// history search.
type searchDirection int

const (
	dirReverse searchDirection = iota
	dirForward
)

// searchBackup snapshots editor state so Ctrl-G can cancel an incremental search.
type searchBackup struct {
	buffer            []string
	lineIndex         int
	bytePointer       int
	historyPointer    int
	hasHistoryPointer bool
	lineBackup        string
	hasLineBackup     bool
}

func (le *LineEditor) viSearchPrev(cmdCtx) { le.incrementalSearchHistory(dirReverse) }
func (le *LineEditor) viSearchNext(cmdCtx) { le.incrementalSearchHistory(dirForward) }

var isearchAcceptKeySyms = map[string]bool{
	"em_delete_prev_char": true, "backward_delete_char": true,
	"vi_search_prev": true, "vi_search_next": true,
	"reverse_search_history": true, "forward_search_history": true,
}

// incrementalSearchHistory begins an incremental search: subsequent keys feed a
// closure (the waiting proc) that grows/shrinks the search word and jumps the
// buffer to matching history (MRI incremental_search_history + generate_searcher).
func (le *LineEditor) incrementalSearchHistory(direction searchDirection) {
	backup := searchBackup{
		buffer:            le.WholeLines(),
		lineIndex:         le.lineIndex,
		bytePointer:       le.bytePointer,
		historyPointer:    le.historyPointer,
		hasHistoryPointer: le.hasHistoryPointer,
		lineBackup:        le.lineBackupHistory,
		hasLineBackup:     le.hasLineBackup,
	}
	searcher := le.generateSearcher(direction)
	promptName := "reverse-i-search"
	if direction == dirForward {
		promptName = "i-search"
	}
	le.searchingPrompt = fmt.Sprintf("(%s)`': ", promptName)
	le.hasSearchingPrompt = true

	terminationKeys := map[string]bool{"\n": true}
	for _, r := range le.config.IsearchTerminators {
		terminationKeys[string(r)] = true
	}

	le.waitingProc = func(k, keySymbol string) {
		if k == "\x07" { // Ctrl-G: cancel and restore
			le.bufferOfLines = backup.buffer
			le.lineIndex = backup.lineIndex
			le.bytePointer = backup.bytePointer
			le.historyPointer = backup.historyPointer
			le.hasHistoryPointer = backup.hasHistoryPointer
			le.lineBackupHistory = backup.lineBackup
			le.hasLineBackup = backup.hasLineBackup
			le.hasSearchingPrompt = false
			le.searchingPrompt = ""
			le.waitingProc = nil
		} else if !terminationKeys[k] && (matchesPrint(k) || isearchAcceptKeySyms[keySymbol]) {
			searchWord, pName, hitPointer, hasHit := searcher(k, keySymbol)
			LastIncrementalSearch = searchWord
			le.searchingPrompt = fmt.Sprintf("(%s)`%s'", pName, searchWord)
			if !le.isMultiline {
				le.searchingPrompt += ": "
			}
			if hasHit {
				le.moveHistory(hitPointer, true, lineEnd, cursorEnd, 0, 0)
			}
		} else {
			ptr := le.historyPointer
			le.moveHistory(ptr, le.hasHistoryPointer, lineEnd, cursorStart, 0, 0)
			le.hasSearchingPrompt = false
			le.searchingPrompt = ""
			le.waitingProc = nil
		}
	}
}

// generateSearcher returns the closure that performs each incremental search
// step, tracking the accumulated search word and direction.
func (le *LineEditor) generateSearcher(direction searchDirection) func(key, keySymbol string) (string, string, int, bool) {
	searchWord := ""
	hitPointer := 0
	return func(key, keySymbol string) (string, string, int, bool) {
		searchAgain := false
		switch keySymbol {
		case "em_delete_prev_char", "backward_delete_char":
			gcs := graphemeClusters(searchWord)
			if len(gcs) > 0 {
				gcs = gcs[:len(gcs)-1]
				searchWord = strings.Join(gcs, "")
			}
		case "reverse_search_history", "vi_search_prev":
			searchAgain = direction == dirReverse
			direction = dirReverse
		case "forward_search_history", "vi_search_next":
			searchAgain = direction == dirForward
			direction = dirForward
		default:
			searchWord += key
		}
		hasHit := false
		hit := ""
		if searchWord != "" && le.hasLineBackup && strings.Contains(le.lineBackupHistory, searchWord) {
			hitPointer = le.hist.Size()
			hit = le.lineBackupHistory
			hasHit = true
			_ = hit
		} else {
			base, history := le.searchHistoryRange(direction, searchAgain, searchWord)
			var hitIndex = -1
			switch direction {
			case dirReverse:
				for i := len(history) - 1; i >= 0; i-- {
					if strings.Contains(history[i], searchWord) {
						hitIndex = i
						break
					}
				}
			case dirForward:
				for i := 0; i < len(history); i++ {
					if strings.Contains(history[i], searchWord) {
						hitIndex = i
						break
					}
				}
			}
			if hitIndex >= 0 {
				hitPointer = base + hitIndex
				hasHit = true
			}
		}
		promptName := "reverse-i-search"
		if direction == dirForward {
			promptName = "i-search"
		}
		if !hasHit {
			promptName = "failed " + promptName
		}
		return searchWord, promptName, hitPointer, hasHit
	}
}

// searchHistoryRange computes the (base offset, slice) of history to scan for a
// step, honoring the search-again repeat semantics and current history pointer.
func (le *LineEditor) searchHistoryRange(direction searchDirection, searchAgain bool, searchWord string) (int, []string) {
	all := le.histSlice(0, le.hist.Size())
	if searchAgain {
		if searchWord == "" && LastIncrementalSearch != "" {
			searchWord = LastIncrementalSearch
		}
		if le.hasHistoryPointer {
			switch direction {
			case dirReverse:
				return 0, le.histSlice(0, le.historyPointer)
			case dirForward:
				return le.historyPointer + 1, le.histSlice(le.historyPointer+1, le.hist.Size())
			}
		}
		return 0, all
	} else if le.hasHistoryPointer {
		switch direction {
		case dirReverse:
			return 0, le.histSlice(0, le.historyPointer+1)
		case dirForward:
			return le.historyPointer, le.histSlice(le.historyPointer, le.hist.Size())
		}
	}
	return 0, all
}

func (le *LineEditor) histSlice(from, to int) []string {
	if from < 0 {
		from = 0
	}
	if to > le.hist.Size() {
		to = le.hist.Size()
	}
	out := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, le.hist.at(i))
	}
	return out
}

func matchesPrint(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// LastIncrementalSearch holds the last incremental search word
// (MRI Reline.last_incremental_search), shared across searches.
var LastIncrementalSearch = ""
