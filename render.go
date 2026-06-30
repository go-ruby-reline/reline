package reline

import "strings"

// RenderState is the pure result of laying out the current buffer for a terminal
// of the given width: the visible (prompt, content) line pairs after word
// wrapping, and the cursor's (x, y) in that wrapped grid. This is the
// "rendering-to-output" computation; writing it to the tty is the IO seam.
type RenderState struct {
	// Lines are wrapped display rows as (prompt, content) pairs.
	Lines [][2]string
	// CursorX is the cursor column within its wrapped row.
	CursorX int
	// CursorY is the cursor row index into Lines.
	CursorY int
}

// PromptProc, when set, maps the buffer lines to per-line prompts (MRI
// prompt_proc). When nil, a single prompt is used for line 0 and "" for the rest.
type PromptProc func(lines []string) []string

// promptList computes the prompt for each buffer line (subset of MRI
// check_multiline_prompt: no mode string / arg / search prompt embedding).
func (le *LineEditor) promptList(promptProc PromptProc) []string {
	buffer := le.bufferOfLines
	if !le.isMultiline {
		out := make([]string, len(buffer))
		out[0] = le.activePrompt()
		return out
	}
	if promptProc != nil {
		pl := promptProc(buffer)
		for i := range pl {
			pl[i] = strings.ReplaceAll(pl[i], "\n", "\\n")
		}
		// MRI: while a numeric argument or incremental search is active, every
		// prompt-proc line is replaced by the active (arg/search) prompt.
		if le.hasViArg || le.hasSearchingPrompt {
			ap := le.activePrompt()
			for i := range pl {
				pl[i] = ap
			}
		}
		if len(pl) == 0 {
			pl = []string{le.activePrompt()}
		}
		for len(pl) < len(buffer) {
			pl = append(pl, pl[len(pl)-1])
		}
		return pl
	}
	out := make([]string, len(buffer))
	for i := range out {
		out[i] = le.activePrompt()
	}
	return out
}

// activePrompt returns the prompt currently in effect: the search prompt during
// incremental search, the arg prompt during a numeric argument, else the base
// prompt (MRI check_multiline_prompt's prompt selection).
func (le *LineEditor) activePrompt() string {
	if le.hasViArg {
		return "(arg: " + itoa(le.viArg) + ") "
	}
	if le.hasSearchingPrompt {
		return le.searchingPrompt
	}
	return le.prompt
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// Render lays out the buffer for a terminal `width` wide using promptProc (which
// may be nil), returning the wrapped lines and cursor position. Pure: no IO.
func (le *LineEditor) Render(width int, promptProc PromptProc) RenderState {
	prompts := le.promptList(promptProc)
	modified := make([]string, len(le.bufferOfLines))
	for i, l := range le.bufferOfLines {
		modified[i] = EscapeForPrint(l)
	}

	var lines [][2]string
	for i := range le.bufferOfLines {
		prompt := prompts[i]
		line := modified[i]
		wrappedPrompts := SplitLineByWidth(prompt, width, 0)
		codeLinePrompt := wrappedPrompts[len(wrappedPrompts)-1]
		wrappedPrompts = wrappedPrompts[:len(wrappedPrompts)-1]
		offset := CalculateWidth(codeLinePrompt, true)
		wrappedLines := SplitLineByWidth(line, width, offset)
		for _, wp := range wrappedPrompts {
			lines = append(lines, [2]string{wp, ""})
		}
		lines = append(lines, [2]string{codeLinePrompt, wrappedLines[0]})
		for _, c := range wrappedLines[1:] {
			lines = append(lines, [2]string{"", c})
		}
	}

	cx, cy := le.wrappedCursorPosition(width, prompts, modified)
	return RenderState{Lines: lines, CursorX: cx, CursorY: cy}
}

// wrappedCursorPosition computes the cursor (x, y) in the wrapped grid
// (MRI wrapped_cursor_position).
func (le *LineEditor) wrappedCursorPosition(width int, prompts, modified []string) (int, int) {
	promptWidth := CalculateWidth(prompts[le.lineIndex], true)
	lineBeforeCursor := EscapeForPrint(byteslice(le.bufferOfLines[le.lineIndex], 0, le.bytePointer))
	wrappedBefore := SplitLineByWidth(strings.Repeat(" ", promptWidth)+lineBeforeCursor, width, 0)

	priorRows := 0
	for i := 0; i < le.lineIndex; i++ {
		prompt := prompts[i]
		line := modified[i]
		wp := SplitLineByWidth(prompt, width, 0)
		codeLinePrompt := wp[len(wp)-1]
		offset := CalculateWidth(codeLinePrompt, true)
		wl := SplitLineByWidth(line, width, offset)
		priorRows += (len(wp) - 1) + len(wl)
	}
	cursorY := priorRows + len(wrappedBefore) - 1
	cursorX := CalculateWidth(wrappedBefore[len(wrappedBefore)-1], false)
	return cursorX, cursorY
}

// History returns the editor's shared history.
func (le *LineEditor) History() *History { return le.hist }

// SearchingPrompt returns the active incremental-search prompt and whether a
// search is in progress (exposed for the embedder's renderer).
func (le *LineEditor) SearchingPrompt() (string, bool) {
	return le.searchingPrompt, le.hasSearchingPrompt
}
