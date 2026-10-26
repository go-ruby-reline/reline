package terminal

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestEnableVT(t *testing.T) {
	gm, sm := getConsoleMode, setConsoleMode
	t.Cleanup(func() { getConsoleMode, setConsoleMode = gm, sm })
	var set uint32
	getConsoleMode = func(windows.Handle, *uint32) error { return errors.New("not a console") }
	setConsoleMode = func(_ windows.Handle, m uint32) error { set = m; return nil }
	if err := enableVT(1); err == nil {
		t.Error("a handle that is not a console")
	}
	getConsoleMode = func(_ windows.Handle, m *uint32) error { *m = windows.ENABLE_PROCESSED_OUTPUT; return nil }
	if err := enableVT(1); err != nil {
		t.Fatal(err)
	}
	if set != windows.ENABLE_PROCESSED_OUTPUT|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING {
		t.Errorf("mode %#x", set)
	}
}
