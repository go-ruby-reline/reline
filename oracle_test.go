package reline

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// rubyOracleScript drives MRI's Reline::LineEditor with the same scripted key
// events and prints the resulting line + byte pointer per scenario, so the Go
// port can be compared byte-for-byte. It is only run when a Reline-capable ruby
// is available (RELINE_RUBY env or a 0.6.x reline on PATH); CI installs ruby on
// the non-Windows lanes. $stdout.binmode keeps Windows from CRLF-polluting the
// captured output.
const rubyOracleScript = `
$stdout.binmode
require 'reline'
scenarios = JSON.parse($stdin.read)
out = scenarios.map do |sc|
  mode = sc['mode']
  config = Reline::Config.new
  config.editing_mode = mode.to_sym if mode
  le = Reline::LineEditor.new(config)
  le.reset(sc['prompt'] || '> ')
  le.multiline_on if sc['multiline']
  le.confirm_multiline_termination_proc = ->(buf) { sc['terminate'] } if sc['multiline']
  sc['keys'].each do |k|
    s = k[0]
    sym = k[1].to_sym
    le.update(Reline::Key.new(s, sym, false))
  end
  { 'line' => le.line, 'ptr' => le.byte_pointer, 'finished' => le.finished?, 'eof' => le.eof? }
end
$stdout.write(JSON.generate(out))
`

// scenario is one scripted editing session shared by the Go test and the MRI
// oracle.
type scenario struct {
	Name      string     `json:"name"`
	Mode      string     `json:"mode,omitempty"`
	Prompt    string     `json:"prompt,omitempty"`
	Multiline bool       `json:"multiline,omitempty"`
	Terminate bool       `json:"terminate,omitempty"`
	Keys      [][]string `json:"keys"`
}

type oracleResult struct {
	Line     *string `json:"line"`
	Ptr      int     `json:"ptr"`
	Finished bool    `json:"finished"`
	EOF      bool    `json:"eof"`
}

func ins(s string) [][]string {
	var out [][]string
	for _, r := range s {
		out = append(out, []string{string(r), "ed_insert"})
	}
	return out
}

func k(s, sym string) []string { return []string{s, sym} }

func keysCat(parts ...[][]string) [][]string {
	var out [][]string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// differentialScenarios is the battery of editing sessions checked against MRI.
func differentialScenarios() []scenario {
	return []scenario{
		{Name: "insert", Keys: ins("hello world")},
		{Name: "insert+home+insert", Keys: keysCat(ins("hello"), [][]string{k("\x01", "ed_move_to_beg"), k("X", "ed_insert")})},
		{Name: "kill-line+yank", Keys: keysCat(ins("foobar"), [][]string{k("\x01", "ed_move_to_beg"), k("\x0b", "ed_kill_line"), k("\x19", "em_yank")})},
		{Name: "kill-word x2 accumulate", Keys: keysCat(ins("alpha beta gamma"), [][]string{k("\x01", "ed_move_to_beg"), k("d", "em_delete_next_word"), k("d", "em_delete_next_word"), k("\x05", "ed_move_to_end"), k("\x19", "em_yank")})},
		{Name: "backward-kill-word prepend", Keys: keysCat(ins("alpha beta"), [][]string{k("\x17", "ed_delete_prev_word"), k("\x17", "ed_delete_prev_word"), k("\x19", "em_yank")})},
		{Name: "transpose-chars", Keys: keysCat(ins("ab"), [][]string{k("\x14", "ed_transpose_chars")})},
		{Name: "transpose-chars-mid", Keys: keysCat(ins("abc"), [][]string{k("\x01", "ed_move_to_beg"), k("\x06", "ed_next_char"), k("\x14", "ed_transpose_chars")})},
		{Name: "transpose-words", Keys: keysCat(ins("foo bar"), [][]string{k("\x05", "ed_move_to_end"), k("t", "ed_transpose_words")})},
		{Name: "upcase-word", Keys: keysCat(ins("foo bar"), [][]string{k("\x01", "ed_move_to_beg"), k("u", "em_upper_case")})},
		{Name: "downcase-word", Keys: keysCat(ins("FOO BAR"), [][]string{k("\x01", "ed_move_to_beg"), k("l", "em_lower_case")})},
		{Name: "capitalize-word", Keys: keysCat(ins("foo bar"), [][]string{k("\x01", "ed_move_to_beg"), k("c", "em_capitol_case")})},
		{Name: "delete-char", Keys: keysCat(ins("hello"), [][]string{k("\x01", "ed_move_to_beg"), k("\x04", "em_delete")})},
		{Name: "backspace", Keys: keysCat(ins("hello"), [][]string{k("\x08", "em_delete_prev_char")})},
		{Name: "unix-line-discard", Keys: keysCat(ins("hello world"), [][]string{k("\x15", "unix_line_discard"), k("\x19", "em_yank")})},
		{Name: "kill-whole-line", Keys: keysCat(ins("xyz"), [][]string{k("k", "em_kill_line")})},
		{Name: "yank-pop", Keys: keysCat(ins("first"), [][]string{k("\x01", "ed_move_to_beg"), k("\x0b", "ed_kill_line")}, ins("second"), [][]string{k("\x01", "ed_move_to_beg"), k("\x0b", "ed_kill_line"), k("\x19", "em_yank"), k("y", "em_yank_pop")})},
		{Name: "forward-word", Keys: keysCat(ins("foo bar baz"), [][]string{k("\x01", "ed_move_to_beg"), k("f", "em_next_word"), k("f", "em_next_word")})},
		{Name: "backward-word", Keys: keysCat(ins("foo bar baz"), [][]string{k("b", "ed_prev_word")})},
		{Name: "multibyte-insert", Keys: [][]string{k("é", "ed_insert"), k("漢", "ed_insert"), k("字", "ed_insert"), k("\x08", "em_delete_prev_char")}},
		{Name: "em-kill-region", Keys: keysCat(ins("foo.bar baz"), [][]string{k("\x17", "em_kill_region")})},
		// vi mode scenarios
		{Name: "vi-insert-esc-move", Mode: "vi_insert", Keys: keysCat(ins("hello"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("l", "ed_next_char")})},
		{Name: "vi-word", Mode: "vi_insert", Keys: keysCat(ins("foo bar baz"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("w", "vi_next_word")})},
		{Name: "vi-end-word", Mode: "vi_insert", Keys: keysCat(ins("foo bar"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("e", "vi_end_word")})},
		{Name: "vi-delete-char", Mode: "vi_insert", Keys: keysCat(ins("hello"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("x", "ed_delete_next_char")})},
		{Name: "vi-append", Mode: "vi_insert", Keys: keysCat(ins("ab"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("a", "vi_add"), k("Z", "ed_insert")})},
		{Name: "vi-cw", Mode: "vi_insert", Keys: keysCat(ins("foo bar"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("c", "vi_change_meta"), k("w", "vi_next_word")})},
		{Name: "vi-dw", Mode: "vi_insert", Keys: keysCat(ins("foo bar"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("d", "vi_delete_meta"), k("w", "vi_next_word")})},
		{Name: "vi-dd", Mode: "vi_insert", Keys: keysCat(ins("foo bar"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("d", "vi_delete_meta"), k("d", "vi_delete_meta")})},
		{Name: "vi-3r", Mode: "vi_insert", Keys: keysCat(ins("hello"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("3", "ed_argument_digit"), k("r", "vi_replace_char"), k("x", "ed_insert")})},
		{Name: "vi-3col", Mode: "vi_insert", Keys: keysCat(ins("hello"), [][]string{k("\x1b", "vi_command_mode"), k("3", "ed_argument_digit"), k("|", "vi_to_column")})},
		{Name: "vi-fchar", Mode: "vi_insert", Keys: keysCat(ins("hello world"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("f", "vi_next_char"), k("o", "ed_insert")})},
		{Name: "vi-bigW", Mode: "vi_insert", Keys: keysCat(ins("foo.bar baz.qux"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("W", "vi_next_big_word")})},
		{Name: "vi-paste", Mode: "vi_insert", Keys: keysCat(ins("hello"), [][]string{k("\x1b", "vi_command_mode"), k("0", "vi_zero"), k("x", "ed_delete_next_char"), k("p", "vi_paste_next")})},
		// numeric argument
		{Name: "arg-forward", Keys: keysCat(ins("abcdef"), [][]string{k("\x01", "ed_move_to_beg"), k("3", "ed_digit"), k("\x06", "ed_next_char")})},
		// multiline
		{Name: "multiline-newline", Multiline: true, Terminate: false, Keys: keysCat(ins("a"), [][]string{k("\n", "ed_newline")}, ins("b"))},
		// submit
		{Name: "submit", Keys: keysCat(ins("done"), [][]string{k("\n", "ed_newline")})},
	}
}

func toScenarioRunner(sc scenario) (*LineEditor, func()) {
	cfg := NewConfig()
	switch sc.Mode {
	case "vi_insert":
		cfg.SetEditingMode(ModeViInsert)
	case "vi_command":
		cfg.SetEditingMode(ModeViCommand)
	}
	io := &NullIO{Rows: 24, Cols: 80}
	le := NewLineEditor(cfg, io, NewHistory(-1))
	prompt := sc.Prompt
	if prompt == "" {
		prompt = "> "
	}
	le.Reset(prompt)
	if sc.Multiline {
		le.MultilineOn()
		term := sc.Terminate
		le.SetConfirmMultilineTermination(func(string) bool { return term })
	}
	return le, func() {
		for _, kv := range sc.Keys {
			le.Update(Key{Char: kv[0], MethodSymbol: kv[1]})
		}
	}
}

func findOracleRuby(t *testing.T) string {
	if r := os.Getenv("RELINE_RUBY"); r != "" {
		return r
	}
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		return ""
	}
	// Confirm reline is present and a compatible version (0.6.x).
	out, err := exec.Command(ruby, "-rreline", "-e", "print Reline::VERSION").CombinedOutput()
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(string(out), "0.6") {
		t.Logf("skipping oracle: reline version %q != 0.6.x", string(out))
		return ""
	}
	return ruby
}

func TestDifferentialVsMRI(t *testing.T) {
	ruby := findOracleRuby(t)
	if ruby == "" {
		t.Skip("no Reline-0.6 ruby available; deterministic tests cover behavior")
	}
	scenarios := differentialScenarios()
	payload, err := json.Marshal(scenarios)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ruby, "-rjson", "-e", rubyOracleScript)
	cmd.Stdin = strings.NewReader(string(payload))
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("ruby oracle failed: %v", err)
	}
	var want []oracleResult
	if err := json.Unmarshal(stdout, &want); err != nil {
		t.Fatalf("decode oracle output %q: %v", stdout, err)
	}
	if len(want) != len(scenarios) {
		t.Fatalf("oracle returned %d results, want %d", len(want), len(scenarios))
	}
	for i, sc := range scenarios {
		le, run := toScenarioRunner(sc)
		run()
		line, ok := le.Line()
		w := want[i]
		var wantLine string
		wantOK := w.Line != nil
		if wantOK {
			wantLine = *w.Line
		}
		if ok != wantOK || (ok && line != wantLine) {
			t.Errorf("[%s] line: got (%q,%v) want (%q,%v)", sc.Name, line, ok, wantLine, wantOK)
		}
		if le.BytePointer() != w.Ptr {
			t.Errorf("[%s] ptr: got %d want %d", sc.Name, le.BytePointer(), w.Ptr)
		}
		if le.Finished() != w.Finished {
			t.Errorf("[%s] finished: got %v want %v", sc.Name, le.Finished(), w.Finished)
		}
		if le.EOF() != w.EOF {
			t.Errorf("[%s] eof: got %v want %v", sc.Name, le.EOF(), w.EOF)
		}
	}
}
