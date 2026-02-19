package utils

import "sync"

// URLTracker tracks visited URLs to avoid duplicates
type URLTracker struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// NewURLTracker creates a new tracker
func NewURLTracker() *URLTracker {
	return &URLTracker{seen: make(map[string]struct{})}
}

// Add returns true if the URL is new (not seen before), false if duplicate
func (t *URLTracker) Add(url string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.seen[url]; exists {
		return false
	}
	t.seen[url] = struct{}{}
	return true
}

// Count returns the number of tracked URLs
func (t *URLTracker) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.seen)
}
