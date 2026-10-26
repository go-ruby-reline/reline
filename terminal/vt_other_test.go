//go:build !windows

package terminal

import "testing"

func TestEnableVTIsANoOp(t *testing.T) {
	if err := enableVT(1); err != nil {
		t.Error(err)
	}
}
