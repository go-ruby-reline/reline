package reline

// killState mirrors Reline::KillRing::State. Consecutive kills accumulate into
// one ring entry while the state stays CONTINUED/PROCESSED; a yank or a fresh
// (non-kill) command resets accumulation.
type killState int

const (
	killFresh killState = iota
	killContinued
	killProcessed
	killYank
)

// ringPoint is a node in the doubly-linked circular kill ring.
type ringPoint struct {
	backward, forward *ringPoint
	str               string
}

// ringBuffer is the fixed-capacity circular buffer of kill entries.
type ringBuffer struct {
	max  int
	size int
	head *ringPoint
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{max: max}
}

func (rb *ringBuffer) push(point *ringPoint) {
	switch {
	case rb.size == 0:
		rb.head = point
		point.backward = point
		point.forward = point
		rb.size = 1
	case rb.size >= rb.max:
		tail := rb.head.forward
		newTail := tail.forward
		rb.head.forward = point
		point.backward = rb.head
		newTail.backward = point
		point.forward = newTail
		rb.head = point
	default:
		tail := rb.head.forward
		rb.head.forward = point
		point.backward = rb.head
		tail.backward = point
		point.forward = tail
		rb.head = point
		rb.size++
	}
}

func (rb *ringBuffer) empty() bool { return rb.size == 0 }

// KillRing is the emacs kill ring (MRI Reline::KillRing): kill commands append
// text, yank pastes the most recent entry, and yank-pop rotates backward.
type KillRing struct {
	ring        *ringBuffer
	ringPointer *ringPoint
	state       killState
}

// NewKillRing returns an empty kill ring holding up to max entries.
func NewKillRing(max int) *KillRing {
	return &KillRing{ring: newRingBuffer(max), state: killFresh}
}

// Append adds killed text. When beforeP is true the text is prepended to the
// current accumulating entry (used by backward kills).
func (k *KillRing) Append(s string, beforeP bool) {
	switch k.state {
	case killFresh, killYank:
		k.ring.push(&ringPoint{str: s})
		k.state = killContinued
	case killContinued, killProcessed:
		if beforeP {
			k.ring.head.str = s + k.ring.head.str
		} else {
			k.ring.head.str = k.ring.head.str + s
		}
		k.state = killContinued
	}
}

// Process advances the kill-accumulation state machine once per dispatched key,
// so that two consecutive kills merge but a kill after another command starts a
// new entry (MRI KillRing#process).
func (k *KillRing) Process() {
	switch k.state {
	case killFresh:
	case killContinued:
		k.state = killProcessed
	case killProcessed:
		k.state = killFresh
	case killYank:
	}
}

// Yank returns the most recent kill entry (or "", false if empty) and enters
// the YANK state so YankPop can rotate.
func (k *KillRing) Yank() (string, bool) {
	if k.ring.empty() {
		return "", false
	}
	k.state = killYank
	k.ringPointer = k.ring.head
	return k.ringPointer.str, true
}

// YankPop rotates to the previous kill entry, returning the new text and the
// previously yanked text (to be replaced). Valid only right after a yank.
func (k *KillRing) YankPop() (yanked, prev string, ok bool) {
	if k.state != killYank {
		return "", "", false
	}
	prevYank := k.ringPointer.str
	k.ringPointer = k.ringPointer.backward
	return k.ringPointer.str, prevYank, true
}

// Each iterates the ring from most-recent to least-recent (MRI KillRing#each).
func (k *KillRing) Each(fn func(string)) {
	start := k.ring.head
	head := start
	for head != nil {
		fn(head.str)
		head = head.backward
		if head == start {
			break
		}
	}
}
