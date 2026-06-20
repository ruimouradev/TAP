// ratelimit.go: a small sliding-window counter for abuse monitoring. The same
// type backs both command-flood detection (one per client) and rapid-connection
// detection (one per IP) - one mechanism, two uses.
package server

import "time"

// rateWindow counts events within a sliding window. It is NOT safe for
// concurrent use; every caller runs inside the Hub goroutine, so no lock is
// needed.
type rateWindow struct {
	count int
	start time.Time
}

// exceeded records one event and reports true exactly once - on the event that
// first crosses limit within window. The window resets once it has elapsed.
func (w *rateWindow) exceeded(now time.Time, limit int, window time.Duration) bool {
	if now.Sub(w.start) > window {
		w.start = now
		w.count = 0
	}
	w.count++
	return w.count == limit+1
}
