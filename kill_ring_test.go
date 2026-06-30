package reline

import "testing"

func TestKillRingBasicYank(t *testing.T) {
	kr := NewKillRing(1024)
	if _, ok := kr.Yank(); ok {
		t.Fatal("empty ring should not yank")
	}
	kr.Append("hello", false)
	got, ok := kr.Yank()
	if !ok || got != "hello" {
		t.Fatalf("yank=%q ok=%v", got, ok)
	}
}

func TestKillRingAccumulate(t *testing.T) {
	kr := NewKillRing(1024)
	kr.Append("foo", false)
	kr.Append("bar", false) // continued -> concat
	got, _ := kr.Yank()
	if got != "foobar" {
		t.Fatalf("accumulate=%q", got)
	}
}

func TestKillRingPrepend(t *testing.T) {
	kr := NewKillRing(1024)
	kr.Append("bar", false)
	kr.Append("foo", true) // prepend
	got, _ := kr.Yank()
	if got != "foobar" {
		t.Fatalf("prepend=%q", got)
	}
}

func TestKillRingProcessSeparatesEntries(t *testing.T) {
	kr := NewKillRing(1024)
	kr.Append("first", false)
	kr.Process() // continued -> processed
	kr.Process() // processed -> fresh
	kr.Append("second", false)
	got, _ := kr.Yank()
	if got != "second" {
		t.Fatalf("yank latest=%q", got)
	}
	y, prev, ok := kr.YankPop()
	if !ok || y != "first" || prev != "second" {
		t.Fatalf("pop=%q prev=%q ok=%v", y, prev, ok)
	}
}

func TestKillRingYankPopRequiresYank(t *testing.T) {
	kr := NewKillRing(1024)
	kr.Append("x", false)
	if _, _, ok := kr.YankPop(); ok {
		t.Fatal("yank_pop without yank should fail")
	}
}

func TestKillRingProcessFreshAndYank(t *testing.T) {
	kr := NewKillRing(1024)
	kr.Process() // fresh -> nothing
	kr.Append("a", false)
	kr.Yank()
	kr.Process() // yank state -> nothing
	// after yank, an append starts a new entry
	kr.Append("b", false)
	got, _ := kr.Yank()
	if got != "b" {
		t.Fatalf("post-yank append=%q", got)
	}
}

func TestKillRingMaxCapacity(t *testing.T) {
	kr := NewKillRing(2)
	kr.Append("a", false)
	kr.Process()
	kr.Process()
	kr.Append("b", false)
	kr.Process()
	kr.Process()
	kr.Append("c", false) // exceeds cap 2 -> oldest dropped
	var entries []string
	kr.Each(func(s string) { entries = append(entries, s) })
	if len(entries) != 2 {
		t.Fatalf("entries=%v", entries)
	}
	if entries[0] != "c" {
		t.Fatalf("head=%q", entries[0])
	}
}

func TestKillRingEachEmpty(t *testing.T) {
	kr := NewKillRing(4)
	count := 0
	kr.Each(func(string) { count++ })
	if count != 0 {
		t.Fatal("empty each")
	}
}
