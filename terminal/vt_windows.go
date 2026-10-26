package terminal

import "golang.org/x/sys/windows"

// The console calls, seams for the tests.
var (
	getConsoleMode = windows.GetConsoleMode
	setConsoleMode = windows.SetConsoleMode
)

// enableVT makes the Windows console behind fd interpret the escape
// sequences written to it (ENABLE_VIRTUAL_TERMINAL_PROCESSING, Windows 10
// and later); term.MakeRaw already makes its input send them.
func enableVT(fd int) error {
	h := windows.Handle(fd)
	var mode uint32
	if err := getConsoleMode(h, &mode); err != nil {
		return err
	}
	return setConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
