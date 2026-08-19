package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	l := New(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow(1) {
			t.Fatalf("expected allow on call %d", i)
		}
	}
	if l.Allow(1) {
		t.Fatal("expected deny after limit reached")
	}
}

func TestAllowResetsAfterWindow(t *testing.T) {
	l := New(2, 10*time.Millisecond)
	if !l.Allow(7) || !l.Allow(7) {
		t.Fatal("expected first two allowed")
	}
	if l.Allow(7) {
		t.Fatal("expected deny at limit")
	}
	time.Sleep(15 * time.Millisecond)
	if !l.Allow(7) {
		t.Fatal("expected allow after window reset")
	}
}

func TestShouldWarnOncePerWindow(t *testing.T) {
	l := New(1, time.Minute)
	_ = l.Allow(9)
	if !l.ShouldWarn(9) {
		t.Fatal("expected first warn true")
	}
	if l.ShouldWarn(9) {
		t.Fatal("expected second warn false within window")
	}
}

func TestCleanupRemovesStale(t *testing.T) {
	l := New(1, 5*time.Millisecond)
	_ = l.Allow(42)
	time.Sleep(10 * time.Millisecond)
	l.Cleanup()
	l.mu.Lock()
	_, ok := l.entries[42]
	l.mu.Unlock()
	if ok {
		t.Fatal("expected stale entry removed by Cleanup")
	}
}
