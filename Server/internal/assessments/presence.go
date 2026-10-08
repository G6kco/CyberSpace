package assessments

import (
	"sync"
	"time"
)

// presence remembers when each student last pinged the test portal. It is
// in memory by design: it only answers "who is in the portal right now",
// and a restart is repaired by the next round of heartbeats.
type presence struct {
	mu   sync.Mutex
	seen map[uint64]time.Time
}

func newPresence() *presence {
	return &presence{seen: map[uint64]time.Time{}}
}

func (p *presence) touch(userID uint64, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen[userID] = now
}

func (p *presence) leave(userID uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.seen, userID)
}

func (p *presence) isOnline(userID uint64, now time.Time, ttl time.Duration) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	seen, ok := p.seen[userID]
	return ok && now.Sub(seen) <= ttl
}

// online returns students seen within ttl and forgets the rest.
func (p *presence) online(now time.Time, ttl time.Duration) map[uint64]time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[uint64]time.Time, len(p.seen))
	for id, seen := range p.seen {
		if now.Sub(seen) > ttl {
			delete(p.seen, id)
			continue
		}
		result[id] = seen
	}
	return result
}

// limiter allows at most limit events per key within window.
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	events map[uint64][]time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, events: map[uint64][]time.Time{}}
}

func (l *limiter) allow(key uint64, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.events[key][:0]
	for _, t := range l.events[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.events[key] = recent
		return false
	}
	l.events[key] = append(recent, now)

	// Keys of finished attempts would otherwise accumulate for the life of
	// the process.
	if len(l.events) > 10000 {
		for k, times := range l.events {
			if len(times) == 0 || now.Sub(times[len(times)-1]) >= l.window {
				delete(l.events, k)
			}
		}
	}
	return true
}
