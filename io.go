package reline

// IO is the terminal host seam. Everything that touches a real tty lives behind
// this interface so the editing core stays pure and fully testable. The
// embedder (e.g. rbgo) supplies a concrete implementation backed by the host
// terminal in raw mode; tests supply a scripted/recording fake.
//
// The pure core only needs three capabilities from the host: read a raw key
// byte, learn the terminal size, and write rendered output. Cursor positioning
// and screen clears are expressed as writes by the renderer, but are surfaced
// here too for embedders that drive the screen directly.
type IO interface {
	// GetC returns the next input byte, or -1 on timeout/EOF. timeoutMs < 0
	// means block indefinitely.
	GetC(timeoutMs int) int
	// UngetC pushes a byte back to be returned by the next GetC.
	UngetC(c int)
	// GetScreenSize returns the terminal size as (rows, cols).
	GetScreenSize() (rows, cols int)
	// Write emits already-rendered output (escape sequences + text) to the tty.
	Write(s string)
	// MoveCursorColumn moves the cursor to an absolute column.
	MoveCursorColumn(col int)
	// EraseAfterCursor clears from the cursor to the end of the line.
	EraseAfterCursor()
	// ClearScreen clears the whole screen.
	ClearScreen()
}

// NullIO is a recording IO with a fixed default 24x80 screen, suitable as a
// default when no real terminal is wired (the pure state machine never blocks on
// it in tests). It captures writes and cursor ops for inspection.
type NullIO struct {
	Rows, Cols int
	Out        []string
	Unget      []int
	Cleared    int
	Col        int
}

// GetC returns a previously ungot byte if any, else reports EOF.
func (n *NullIO) GetC(timeoutMs int) int {
	if len(n.Unget) > 0 {
		c := n.Unget[len(n.Unget)-1]
		n.Unget = n.Unget[:len(n.Unget)-1]
		return c
	}
	return -1
}

// UngetC pushes a byte back to be returned by the next GetC.
func (n *NullIO) UngetC(c int) { n.Unget = append(n.Unget, c) }

// GetScreenSize returns the configured size (defaulting to 24x80).
func (n *NullIO) GetScreenSize() (int, int) {
	rows, cols := n.Rows, n.Cols
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	return rows, cols
}

// Write records output for inspection.
func (n *NullIO) Write(s string) { n.Out = append(n.Out, s) }

// MoveCursorColumn records the requested cursor column.
func (n *NullIO) MoveCursorColumn(col int) { n.Col = col }

// EraseAfterCursor records an erase request as a sentinel write.
func (n *NullIO) EraseAfterCursor() { n.Out = append(n.Out, "\x1b[K") }

// ClearScreen records a screen clear.
func (n *NullIO) ClearScreen() { n.Cleared++ }
