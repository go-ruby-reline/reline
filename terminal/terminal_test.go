package terminal

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"golang.org/x/term"

	"github.com/go-ruby-reline/reline"
)

// fake replaces the terminal device: raw mode succeeds and is undone, the
// size is fixed.
type fake struct {
	mu        sync.Mutex
	raw, rest int
	rawErr    error
	w, h      int
	sizeErr   error
}

func (f *fake) install(t *testing.T) {
	t.Helper()
	mr, re, gs, ev := makeRaw, restore, getSize, enableVTx
	t.Cleanup(func() { makeRaw, restore, getSize, enableVTx = mr, re, gs, ev })
	makeRaw = func(int) (*term.State, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.rawErr != nil {
			return nil, f.rawErr
		}
		f.raw++
		return &term.State{}, nil
	}
	restore = func(int, *term.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.rest++
		return nil
	}
	getSize = func(int) (int, int, error) { return f.w, f.h, f.sizeErr }
	enableVTx = func(int) error { return nil }
}

func newFake(t *testing.T, input string) (*Terminal, *bytes.Buffer, *fake) {
	t.Helper()
	f := &fake{w: 80, h: 24}
	f.install(t)
	var out bytes.Buffer
	return newTerminal(strings.NewReader(input), &out, 0, 1), &out, f
}

func TestReadline(t *testing.T) {
	tt, out, f := newFake(t, "puts 1\r")
	line, err := tt.Readline(Options{Prompt: "> ", AddHistory: true})
	if err != nil || line != "puts 1" {
		t.Fatalf("%q %v", line, err)
	}
	if f.raw != 1 || f.rest != 1 {
		t.Errorf("raw %d, restored %d", f.raw, f.rest)
	}
	if tt.History.Size() != 1 {
		t.Errorf("history %d", tt.History.Size())
	}
	s := out.String()
	if !strings.HasPrefix(s, "\x1b[?2004h\r\x1b[J> \r\x1b[2C") || !strings.HasSuffix(s, "> puts 1\r\x1b[8C\r\n\x1b[?2004l") {
		t.Errorf("drawing %q", s)
	}
}

func TestReadlineEditingAndHistory(t *testing.T) {
	// "ab", Left, "X", Home, "Y", Enter; then Up recalls it, End, "!", Enter.
	tt, _, _ := newFake(t, "ab\x1b[DX\x1b[HY\r\x1b[A\x1b[F!\r")
	line, err := tt.Readline(Options{Prompt: "> ", AddHistory: true})
	if err != nil || line != "YaXb" {
		t.Fatalf("%q %v", line, err)
	}
	line, err = tt.Readline(Options{Prompt: "> "})
	if err != nil || line != "YaXb!" {
		t.Fatalf("recalled %q %v", line, err)
	}
	if tt.History.Size() != 1 {
		t.Errorf("without AddHistory the history grew: %d", tt.History.Size())
	}
}

func TestReadlineEmptyIsNotHistory(t *testing.T) {
	tt, _, _ := newFake(t, "\r")
	if line, err := tt.Readline(Options{AddHistory: true}); err != nil || line != "" {
		t.Fatalf("%q %v", line, err)
	}
	if tt.History.Size() != 0 {
		t.Error("an empty line went into the history")
	}
}

func TestReadlineMultiline(t *testing.T) {
	complete := func(buf string) bool { return strings.Count(buf, "end") >= strings.Count(buf, "def") }
	prompts := func(lines []string) []string {
		out := make([]string, len(lines))
		for i := range out {
			out[i] = "* "
		}
		out[0] = "> "
		return out
	}
	tt, out, _ := newFake(t, "def f\r1\rend\r")
	line, err := tt.Readline(Options{Prompt: "> ", Terminate: complete, PromptProc: prompts, AddHistory: true})
	if err != nil || line != "def f\n1\nend" {
		t.Fatalf("%q %v", line, err)
	}
	// The last drawing starts two rows up from the cursor on the third line.
	if !strings.Contains(out.String(), "\x1b[2A\r\x1b[J> def f\r\n* 1\r\n* end") {
		t.Errorf("drawing %q", out.String())
	}
	if h, _ := tt.History.Get(0); h != "def f\n1\nend" {
		t.Errorf("history %q", h)
	}
}

func TestReadlineCompletionMenu(t *testing.T) {
	cands := func(target, pre, post string) []string { return []string{"foo", "food", "fool"} }
	tt, out, _ := newFake(t, "f\t\t\r")
	line, err := tt.Readline(Options{Prompt: "> ", Complete: cands, AppendCharacter: " "})
	if err != nil || line != "foo" {
		t.Fatalf("%q %v", line, err)
	}
	// The menu is drawn below the line, then the cursor goes back up to it.
	if !strings.Contains(out.String(), "> foo\r\nfoo   food  fool\x1b[1A\r\x1b[5C") {
		t.Errorf("drawing %q", out.String())
	}
	// The next drawing starts from the line, and erases the menu.
	if !strings.Contains(out.String(), "\x1b[5C\r\x1b[J> foo\r") {
		t.Errorf("redraw %q", out.String())
	}
}

func TestReadlineUniqueCompletion(t *testing.T) {
	cands := func(target, pre, post string) []string { return []string{"Stats"} }
	tt, _, _ := newFake(t, "St\t\r")
	if line, _ := tt.Readline(Options{Complete: cands, AppendCharacter: " "}); line != "Stats " {
		t.Errorf("%q", line)
	}
}

func TestReadlineInterrupt(t *testing.T) {
	tt, out, f := newFake(t, "abc\x03")
	if _, err := tt.Readline(Options{Prompt: "> "}); !errors.Is(err, ErrInterrupt) {
		t.Fatalf("err %v", err)
	}
	if !strings.HasSuffix(out.String(), "> abc^C\r\n\x1b[?2004l") {
		t.Errorf("drawing %q", out.String())
	}
	if f.rest != 1 {
		t.Error("raw mode not undone")
	}
}

func TestReadlineEOF(t *testing.T) {
	// Ctrl-D on an empty line.
	tt, _, _ := newFake(t, "\x04")
	if _, err := tt.Readline(Options{}); err != io.EOF {
		t.Fatalf("Ctrl-D: %v", err)
	}
	// The input ends: on an empty buffer, the end; and every read after it.
	tt, _, _ = newFake(t, "")
	for range 2 {
		if _, err := tt.Readline(Options{}); err != io.EOF {
			t.Fatalf("end: %v", err)
		}
	}
	// The input ends after some text: the text is the line, as in MRI.
	tt, _, _ = newFake(t, "1 + 1")
	if line, err := tt.Readline(Options{}); err != nil || line != "1 + 1" {
		t.Fatalf("partial: %q %v", line, err)
	}
}

func TestReadlineRawModeFails(t *testing.T) {
	tt, _, f := newFake(t, "x\r")
	f.rawErr = errors.New("not a terminal")
	if _, err := tt.Readline(Options{}); err == nil || !strings.Contains(err.Error(), "not a terminal") {
		t.Fatalf("err %v", err)
	}
}

func TestReadlineBracketedPaste(t *testing.T) {
	complete := func(buf string) bool { return true }
	tt, _, _ := newFake(t, "x = "+"\x1b[200~[1,\r2]\x1b[201~"+"\r")
	if line, err := tt.Readline(Options{Terminate: complete}); err != nil || line != "x = [1,\n2]" {
		t.Fatalf("%q %v", line, err)
	}
	// Text that is not UTF-8 is dropped.
	tt, _, _ = newFake(t, "\x1b[200~\xff\x1b[201~ok\r")
	if line, _ := tt.Readline(Options{}); line != "ok" {
		t.Errorf("invalid paste: %q", line)
	}
	// A paste the input cuts short gives what came.
	tt, _, _ = newFake(t, "\x1b[200~abc")
	if line, _ := tt.Readline(Options{}); line != "abc" {
		t.Errorf("cut paste: %q", line)
	}
}

func TestReadlineClearScreen(t *testing.T) {
	tt, out, _ := newFake(t, "a\x0c\r")
	if line, _ := tt.Readline(Options{Prompt: "> "}); line != "a" {
		t.Fatalf("%q", line)
	}
	if !strings.Contains(out.String(), "\x1b[2J\x1b[1;1H\r\x1b[J> a") {
		t.Errorf("drawing %q", out.String())
	}
}

func TestReadlineWraps(t *testing.T) {
	// 10 columns: "> " and 12 characters take two rows; the cursor is on the
	// second, and the next drawing goes back up one row.
	tt, out, f := newFake(t, "abcdefghijkl\r")
	f.w = 10
	if line, _ := tt.Readline(Options{Prompt: "> "}); line != "abcdefghijkl" {
		t.Fatalf("%q", line)
	}
	if !strings.Contains(out.String(), "\x1b[1A\r\x1b[J> abcdefgh\r\nijkl\r\x1b[4C\r\n") {
		t.Errorf("drawing %q", out.String())
	}
}

func TestGetC(t *testing.T) {
	r, w := io.Pipe()
	f := &fake{w: 80, h: 24}
	f.install(t)
	tt := newTerminal(r, io.Discard, 0, 1)
	tt.startReader()
	tt.startReader() // once
	if c := tt.GetC(5); c != -1 {
		t.Errorf("timeout: %d", c)
	}
	go func() { _, _ = w.Write([]byte("z")) }()
	if c := tt.GetC(10_000); c != 'z' {
		t.Errorf("timed read: %d", c)
	}
	tt.UngetC('y')
	if c := tt.GetC(0); c != 'y' {
		t.Errorf("pushed back: %d", c)
	}
	_ = w.Close()
	if c := tt.GetC(10_000); c != -1 {
		t.Errorf("timed read at the end: %d", c)
	}
	if c := tt.GetC(-1); c != -1 {
		t.Errorf("read at the end: %d", c)
	}
}

func TestGetScreenSize(t *testing.T) {
	tt, _, f := newFake(t, "")
	if r, c := tt.GetScreenSize(); r != 24 || c != 80 {
		t.Errorf("terminal: %d %d", r, c)
	}
	f.sizeErr = errors.New("no size")
	env := map[string]string{"LINES": "40", "COLUMNS": "120"}
	old := getenv
	t.Cleanup(func() { getenv = old })
	getenv = func(k string) string { return env[k] }
	if r, c := tt.GetScreenSize(); r != 40 || c != 120 {
		t.Errorf("environment: %d %d", r, c)
	}
	env = nil
	if r, c := tt.GetScreenSize(); r != 24 || c != 80 {
		t.Errorf("default: %d %d", r, c)
	}
	f.sizeErr, f.w = nil, 0
	if r, c := tt.GetScreenSize(); r != 24 || c != 80 {
		t.Errorf("zero size: %d %d", r, c)
	}
}

func TestIOMethods(t *testing.T) {
	tt, out, _ := newFake(t, "")
	tt.Write("w")
	tt.MoveCursorColumn(4)
	tt.EraseAfterCursor()
	if got := out.String(); got != "w\x1b[5G\x1b[K" {
		t.Errorf("%q", got)
	}
}

func TestNew(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	tt := New(r, w)
	if tt.inFd != int(r.Fd()) || tt.outFd != int(w.Fd()) || tt.Config == nil || tt.History == nil {
		t.Errorf("%+v", tt)
	}
	// The ANSI bindings are in its keymap: Up is a history move.
	in := &reline.NullIO{}
	for _, c := range []int{'A', '[', 0x1b} {
		in.UngetC(c)
	}
	want := []reline.Key{{Char: "\x1b[A", MethodSymbol: "ed_prev_history"}}
	if got := reline.ReadKeys(tt.Config, in); !reflect.DeepEqual(got, want) {
		t.Errorf("Up: %+v", got)
	}
}
