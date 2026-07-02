// ratelimit.go: a small sliding window counter for abuse monitoring. The same
// type is used for command flood detection per client and rapid connection
// detection per IP. One mechanism, two uses.
package server

import "time"

// rateWindow counts events inside a sliding window. It is not safe for
// concurrent use. Every caller runs inside the Hub goroutine, so no lock
// is needed.
type rateWindow struct {
	count int
	start time.Time
}

// exceeded records one event and reports true exactly once, on the event that
// first crosses the limit inside the window. The window resets after it elapses.
func (w *rateWindow) exceeded(now time.Time, limit int, window time.Duration) bool {
	if now.Sub(w.start) > window {
		w.start = now
		w.count = 0
	}
	w.count++
	return w.count == limit+1
}
