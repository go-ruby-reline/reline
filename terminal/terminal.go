// Package terminal runs the reline line editor on a real terminal: the host
// half of MRI's Reline (Reline::Core#readline and #readmultiline over
// Reline::ANSI) that the editing core leaves to its embedder.
//
// A Terminal puts its input in raw mode for the time of one Readline, decodes
// the keys it reads with the editor's keymaps and the ANSI terminal bindings
// (reline.SetANSIKeyBindings), feeds them to a reline.LineEditor, and redraws
// the edited lines, with their prompts and the completion menu, after every
// key. Between two reads the terminal is left as it was found, so a program's
// output is written in the terminal's normal mode.
//
// It is pure Go: raw mode and the window size come from golang.org/x/term,
// on Unix and on Windows (whose console then takes virtual-terminal input and
// output sequences, Windows 10 and later).
package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/go-ruby-reline/reline"
)

// ErrInterrupt is returned by Readline when Ctrl-C is typed: the read is
// abandoned, as MRI's Reline raises Interrupt.
var ErrInterrupt = errors.New("interrupt")

// Options configure one Readline.
type Options struct {
	// Prompt is shown before the first line.
	Prompt string

	// Terminate, when set, makes the read multiline (MRI
	// Reline.readmultiline): Enter on the last line submits the buffer when
	// Terminate accepts it — the buffer is given with a final newline — and
	// starts a new line of the buffer otherwise.
	Terminate func(buffer string) bool

	// PromptProc gives each line of a multiline buffer its prompt (MRI
	// Reline.prompt_proc); nil shows Prompt on every line.
	PromptProc reline.PromptProc

	// Complete computes the completion candidates for the word before the
	// cursor (MRI Reline.completion_proc); nil disables completion.
	Complete reline.CompletionProc

	// AppendCharacter is added after a unique completion, typically a space
	// (MRI Reline.completion_append_character).
	AppendCharacter string

	// AddHistory appends a submitted, non-empty buffer to the history (the
	// add_hist argument of MRI's readline).
	AddHistory bool
}

// Terminal is a reline.IO on a terminal. Its zero value is not usable; make
// one with New.
type Terminal struct {
	// Config holds the keymaps and the editing mode; New gives it the ANSI
	// terminal bindings.
	Config *reline.Config
	// History is the history the editor navigates; New makes it unlimited.
	History *reline.History

	in      io.Reader
	out     io.Writer
	inFd    int
	outFd   int
	bytes   chan int
	started bool
	unget   []int
	cursorY int // the row of the cursor in the last drawing, from its top
}

// Seams the tests replace: what touches the terminal device itself.
var (
	makeRaw   = term.MakeRaw
	restore   = term.Restore
	getSize   = term.GetSize
	getenv    = os.Getenv
	enableVTx = enableVT
)

// New returns a Terminal reading keys from in and drawing on out, which are
// normally os.Stdin and os.Stdout; in must be a terminal (term.IsTerminal).
func New(in, out *os.File) *Terminal {
	return newTerminal(in, out, int(in.Fd()), int(out.Fd()))
}

func newTerminal(in io.Reader, out io.Writer, inFd, outFd int) *Terminal {
	cfg := reline.NewConfig()
	reline.SetANSIKeyBindings(cfg)
	return &Terminal{
		Config:  cfg,
		History: reline.NewHistory(-1),
		in:      in,
		out:     out,
		inFd:    inFd,
		outFd:   outFd,
		bytes:   make(chan int, 4096),
	}
}

// Readline reads one buffer: a line, or several with o.Terminate. It returns
// io.EOF when Ctrl-D is typed on an empty buffer or the input ends,
// ErrInterrupt on Ctrl-C, and an error when the terminal cannot be put in
// raw mode. The submitted buffer has no final newline.
func (t *Terminal) Readline(o Options) (string, error) {
	state, err := makeRaw(t.inFd)
	if err != nil {
		return "", fmt.Errorf("terminal: raw mode: %w", err)
	}
	defer func() { _ = restore(t.inFd, state) }()
	_ = enableVTx(t.outFd) // without it a Windows console shows the sequences; other systems need nothing
	t.startReader()
	t.write("\x1b[?2004h") // bracketed paste on
	defer t.write("\x1b[?2004l")

	le := reline.NewLineEditor(t.Config, t, t.History)
	le.Reset(o.Prompt)
	if o.Terminate != nil {
		le.MultilineOn()
		le.SetConfirmMultilineTermination(o.Terminate)
	}
	if o.Complete != nil {
		le.SetCompletionProc(o.Complete)
	}
	le.SetCompletionAppendCharacter(o.AppendCharacter)
	t.cursorY = 0
	t.render(le, o.PromptProc, "")
	for !le.Finished() {
		for _, key := range reline.ReadKeys(t.Config, t) {
			switch {
			case key.Char == "\x03":
				t.render(le, o.PromptProc, "^C")
				t.write("\r\n")
				return "", ErrInterrupt
			case key.MethodSymbol == "bracketed_paste_start":
				le.InsertMultilineText(t.readPaste())
			default:
				le.Update(key)
			}
		}
		if !le.Finished() {
			t.render(le, o.PromptProc, "")
		}
	}
	t.render(le, o.PromptProc, "")
	t.write("\r\n")
	line, ok := le.Line()
	if !ok {
		return "", io.EOF
	}
	if o.AddHistory && strings.TrimRight(line, "\n") != "" {
		t.History.Push(line)
	}
	return line, nil
}

// startReader starts the goroutine that reads the input, once: the bytes
// typed while no Readline runs wait for the next one, as a terminal's own
// type-ahead does.
func (t *Terminal) startReader() {
	if t.started {
		return
	}
	t.started = true
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := t.in.Read(buf)
			for _, b := range buf[:n] {
				t.bytes <- int(b)
			}
			if err != nil {
				close(t.bytes)
				return
			}
		}
	}()
}

// readPaste reads a bracketed paste up to its end marker (MRI
// Reline::ANSI#read_bracketed_paste); text that is not valid UTF-8 is
// dropped, and an input that ends first gives what was read.
func (t *Terminal) readPaste() string {
	var b strings.Builder
	for !strings.HasSuffix(b.String(), reline.EndBracketedPaste) {
		c := t.GetC(-1)
		if c < 0 {
			break
		}
		b.WriteByte(byte(c))
	}
	s := strings.TrimSuffix(b.String(), reline.EndBracketedPaste)
	if !utf8.ValidString(s) {
		return ""
	}
	return s
}

// render redraws the buffer in place of the last drawing: the wrapped lines
// with their prompts, the pending completion menu below them, and the cursor
// where the editor has it. suffix is written after the last line (the ^C of
// an interrupted read); a drawing with a suffix leaves the cursor at its end.
func (t *Terminal) render(le *reline.LineEditor, prompts reline.PromptProc, suffix string) {
	_, cols := t.GetScreenSize()
	rs := le.Render(cols, prompts)
	var menu []string
	if list, ok := le.TakeMenu(); ok {
		menu = reline.MenuLines(list, cols)
	}
	var b strings.Builder
	if t.cursorY > 0 {
		b.WriteString("\x1b[" + strconv.Itoa(t.cursorY) + "A")
	}
	b.WriteString("\r\x1b[J")
	for i, row := range rs.Lines {
		if i > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString(row[0] + row[1])
	}
	for _, m := range menu {
		b.WriteString("\r\n" + m)
	}
	last := len(rs.Lines) + len(menu) - 1
	if suffix != "" {
		b.WriteString(suffix)
		t.cursorY = last
		t.write(b.String())
		return
	}
	if up := last - rs.CursorY; up > 0 {
		b.WriteString("\x1b[" + strconv.Itoa(up) + "A")
	}
	b.WriteString("\r")
	if rs.CursorX > 0 {
		b.WriteString("\x1b[" + strconv.Itoa(rs.CursorX) + "C")
	}
	t.cursorY = rs.CursorY
	t.write(b.String())
}

// write writes s to the terminal; a terminal that cannot be written to has
// nothing to show the error on.
func (t *Terminal) write(s string) { _, _ = io.WriteString(t.out, s) }

// GetC implements reline.IO: the next input byte, a byte pushed back first;
// -1 when the input has ended or, with timeoutMs >= 0, when no byte comes
// within timeoutMs milliseconds.
func (t *Terminal) GetC(timeoutMs int) int {
	if n := len(t.unget); n > 0 {
		c := t.unget[n-1]
		t.unget = t.unget[:n-1]
		return c
	}
	if timeoutMs < 0 {
		c, ok := <-t.bytes
		if !ok {
			return -1
		}
		return c
	}
	timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer timer.Stop()
	select {
	case c, ok := <-t.bytes:
		if !ok {
			return -1
		}
		return c
	case <-timer.C:
		return -1
	}
}

// UngetC implements reline.IO: c is the next byte GetC returns.
func (t *Terminal) UngetC(c int) { t.unget = append(t.unget, c) }

// GetScreenSize implements reline.IO (MRI Reline::ANSI#get_screen_size):
// the terminal's size, else LINES and COLUMNS, else 24 by 80.
func (t *Terminal) GetScreenSize() (rows, cols int) {
	if w, h, err := getSize(t.outFd); err == nil && w > 0 && h > 0 {
		return h, w
	}
	r, _ := strconv.Atoi(getenv("LINES"))
	c, _ := strconv.Atoi(getenv("COLUMNS"))
	if r > 0 && c > 0 {
		return r, c
	}
	return 24, 80
}

// Write implements reline.IO.
func (t *Terminal) Write(s string) { t.write(s) }

// MoveCursorColumn implements reline.IO.
func (t *Terminal) MoveCursorColumn(col int) { t.write("\x1b[" + strconv.Itoa(col+1) + "G") }

// EraseAfterCursor implements reline.IO.
func (t *Terminal) EraseAfterCursor() { t.write("\x1b[K") }

// ClearScreen implements reline.IO (MRI Reline::ANSI#clear_screen): the
// screen is cleared and the next drawing starts at its top.
func (t *Terminal) ClearScreen() {
	t.write("\x1b[2J\x1b[1;1H")
	t.cursorY = 0
}
