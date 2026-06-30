<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-reline/brand/main/social/go-ruby-reline-reline.png" alt="go-ruby-reline/reline" width="720"></p>

# reline — go-ruby-reline

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-reline.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [Reline](https://github.com/ruby/reline)
line-editor core** — the deterministic editing state machine of MRI 4.0.5's
`Reline::LineEditor`. It drives a multibyte, rune-aware line buffer through the
full emacs + vi command set, history navigation and incremental search, the
kill-ring, completion, and the pure rendering computation — **without any Ruby
runtime and without a real terminal**.

It is the line-editor backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (the prerequisite
for a pure-Go IRB), but is a **standalone, reusable** module — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler), and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych port).

> **What it is — and isn't.** The editing semantics — how each keystroke mutates
> the buffer and cursor, how the keymaps dispatch escape/control sequences to
> commands, how history and completion behave, and how the buffer is laid out for
> a terminal of a given width — are fully deterministic and need **no
> interpreter and no tty**, so they live here as pure Go. The terminal raw-mode
> I/O (reading raw key bytes, learning the window size, writing the rendered
> output) is a small host **seam** (`IO`) the embedder wires to the real
> terminal.

## What's implemented

A faithful port of `Reline::LineEditor` (and `KillRing`, `History`, `KeyStroke`,
`KeyActor`, the `Unicode` width/word logic), validated key-for-key against the
`ruby -rreline` oracle:

- **Editing commands** — insert; backward/forward char and word; beginning/end
  of line; delete char/word; kill-line, kill-whole-line, unix-line-discard,
  kill-word, backward-kill-word, kill-region; yank + yank-pop (the kill ring);
  transpose chars/words; up/down/capitalize word; undo/redo.
- **emacs + vi keymaps** — the exact 256-entry `EMACS_MAPPING`,
  `VI_COMMAND_MAPPING`, and `VI_INSERT_MAPPING` arrays from MRI, the
  ESC/CSI/SS3 byte-sequence matcher, numeric arguments, and the vi operator
  grammar (`cw`, `dw`, `dd`, `2cw`, `df<c>`, `r`/`3r`, `f`/`t`/`F`/`T`, `p`/`P`,
  `W`/`E`/`B`, `J`, …).
- **History** — `Reline::HISTORY` (size-capped, with the MRI range/index
  errors), up/down navigation, prefix search, and incremental search (Ctrl-R /
  Ctrl-S) with the failed/cancel/repeat states.
- **Completion** — the completion-proc seam, common-prefix completion, the
  perfect-match/menu state machine, and the autocompletion journey (cycling).
- **Multiline** edit state with a termination-predicate seam, plus undo/redo.
- **Rendering (the pure part)** — given the buffer, cursor, prompt(s), and
  terminal width, `Render` computes the word-wrapped `(prompt, content)` rows and
  the cursor `(x, y)`; writing that to the tty is the host's job.

## The terminal-I/O seam

Everything that touches a real terminal lives behind the `IO` interface, so the
editing core stays pure and fully testable by feeding scripted key events:

```go
type IO interface {
    GetC(timeoutMs int) int        // read one raw input byte (-1 = EOF/timeout)
    UngetC(c int)                  // push a byte back
    GetScreenSize() (rows, cols int)
    Write(s string)                // emit rendered output to the tty
    MoveCursorColumn(col int)
    EraseAfterCursor()
    ClearScreen()
}
```

A `NullIO` (fixed 24×80, recording) ships for tests and headless use.

## Usage

```go
cfg := reline.NewConfig()                  // emacs mode, unlimited history
hist := reline.NewHistory(-1)
le := reline.NewLineEditor(cfg, &reline.NullIO{}, hist)
le.Reset("> ")

// feed scripted key events (the embedder decodes raw bytes via KeyStroke)
for _, ev := range []reline.Key{
    {Char: "h", MethodSymbol: "ed_insert"},
    {Char: "i", MethodSymbol: "ed_insert"},
    {Char: "\x01", MethodSymbol: "ed_move_to_beg"},
    {Char: "X", MethodSymbol: "ed_insert"},
} {
    le.Update(ev)
}
line, _ := le.Line() // "Xhi"

rs := le.Render(80, nil) // wrapped lines + cursor position (pure; no tty)
```

## Tests & coverage

- **100% statement coverage** (including every error branch), enforced in CI.
- **Differential oracle** — a battery of scripted editing sessions is replayed
  through `ruby -rreline` and compared byte-for-byte against this port
  (line + cursor + finished/eof). The oracle self-skips on Windows and where a
  Reline-0.6 ruby is absent; the deterministic, ruby-free tests alone keep
  coverage at 100% so every lane passes.
- **6 architectures** (amd64/arm64/riscv64/loong64/ppc64le/s390x) and
  **3 operating systems** (Linux/macOS/Windows).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-reline/reline authors.
