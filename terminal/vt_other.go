//go:build !windows

package terminal

// enableVT has nothing to do outside Windows: a terminal reads the escape
// sequences of its output itself.
func enableVT(int) error { return nil }
