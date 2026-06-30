package reline

import "fmt"

// HistoryError is raised for out-of-range/out-of-bounds history index access,
// mirroring MRI's RangeError / IndexError from Reline::History#check_index.
type HistoryError struct{ msg string }

func (e *HistoryError) Error() string { return e.msg }

// History is the line-edit history (MRI Reline::History, a size-capped Array).
// MaxSize > 0 caps the number of entries (oldest dropped); 0 drops everything;
// negative means unlimited.
type History struct {
	entries []string
	MaxSize int
}

// NewHistory returns an empty history with the given size cap (-1 = unlimited,
// the MRI default).
func NewHistory(maxSize int) *History {
	return &History{MaxSize: maxSize}
}

// Size returns the number of entries.
func (h *History) Size() int { return len(h.entries) }

// Empty reports whether the history has no entries.
func (h *History) Empty() bool { return len(h.entries) == 0 }

// Clear removes all entries.
func (h *History) Clear() { h.entries = nil }

// Push appends one or more entries, enforcing the size cap. Mirrors
// Reline::History#push (and #concat, which is push applied per element).
func (h *History) Push(vals ...string) {
	if h.MaxSize == 0 {
		return
	}
	if h.MaxSize > 0 {
		diff := len(h.entries) + len(vals) - h.MaxSize
		if diff > 0 {
			if diff <= len(h.entries) {
				h.entries = h.entries[diff:]
			} else {
				// Dropping every existing entry plus the overflow from vals.
				// diff-len(entries) == len(vals)-MaxSize <= len(vals) (MaxSize>0),
				// so the surviving slice is always in range.
				diff -= len(h.entries)
				h.entries = nil
				vals = vals[diff:]
			}
		}
	}
	h.entries = append(h.entries, vals...)
}

// Append adds a single entry (MRI Reline::History#<<).
func (h *History) Append(val string) {
	if h.MaxSize == 0 {
		return
	}
	if h.MaxSize > 0 && len(h.entries)+1 > h.MaxSize {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, val)
}

// checkIndex normalizes a possibly-negative index and enforces MRI's bounds.
func (h *History) checkIndex(index int) (int, error) {
	if index < 0 {
		index += len(h.entries)
	}
	if index < -2147483648 || 2147483647 < index {
		return 0, &HistoryError{fmt.Sprintf("integer %d too big to convert to 'int'", index)}
	}
	if h.MaxSize > 0 {
		if index < -h.MaxSize || h.MaxSize < index {
			return 0, &HistoryError{fmt.Sprintf("index=<%d>", index)}
		}
	}
	if index < 0 || len(h.entries) <= index {
		return 0, &HistoryError{fmt.Sprintf("index=<%d>", index)}
	}
	return index, nil
}

// Get returns the entry at index (supporting negative indices), mirroring
// Reline::History#[].
func (h *History) Get(index int) (string, error) {
	i, err := h.checkIndex(index)
	if err != nil {
		return "", err
	}
	return h.entries[i], nil
}

// Set replaces the entry at index (MRI Reline::History#[]=).
func (h *History) Set(index int, val string) error {
	i, err := h.checkIndex(index)
	if err != nil {
		return err
	}
	h.entries[i] = val
	return nil
}

// DeleteAt removes the entry at index, returning it (MRI #delete_at).
func (h *History) DeleteAt(index int) (string, error) {
	i, err := h.checkIndex(index)
	if err != nil {
		return "", err
	}
	v := h.entries[i]
	h.entries = append(h.entries[:i], h.entries[i+1:]...)
	return v, nil
}

// at returns the entry at a normalized in-bounds index without MaxSize checks,
// for the editor's internal navigation (which already bounds the pointer).
func (h *History) at(index int) string {
	return h.entries[index]
}

// setAt assigns the entry at a normalized in-bounds index (internal use).
func (h *History) setAt(index int, val string) {
	h.entries[index] = val
}
