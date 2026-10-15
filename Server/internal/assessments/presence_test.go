package assessments

import (
	"testing"
	"time"
)

func TestPresenceExpiresAfterTTL(t *testing.T) {
	p := newPresence()
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	p.touch(1, start)
	p.touch(2, start.Add(20*time.Second))

	now := start.Add(40 * time.Second)
	if p.isOnline(1, now, 30*time.Second) {
		t.Error("student 1 still online 40s after the last heartbeat")
	}
	if !p.isOnline(2, now, 30*time.Second) {
		t.Error("student 2 offline 20s after the last heartbeat")
	}
	if online := p.online(now, 30*time.Second); len(online) != 1 {
		t.Fatalf("online() = %v; want only student 2", online)
	}
	p.leave(2)
	if p.isOnline(2, now, 30*time.Second) {
		t.Error("student 2 still online after leaving")
	}
}

func TestLimiterAllowsLimitPerWindow(t *testing.T) {
	l := newLimiter(3, time.Minute)
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	for i := range 3 {
		if !l.allow(7, now.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("event %d refused; want allowed", i+1)
		}
	}
	if l.allow(7, now.Add(3*time.Second)) {
		t.Fatal("fourth event in the window allowed")
	}
	if !l.allow(8, now) {
		t.Fatal("another attempt was limited by this one")
	}
	if !l.allow(7, now.Add(time.Minute+time.Second)) {
		t.Fatal("event after the window refused")
	}
}
