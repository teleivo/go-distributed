// Package rate implements a few different rate limiters aimed to protect API
// endpoints from excessive requests.
package rate

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// TODO should I remove the monotonic clock from the reset and current time?
// if I send the reset time to a client, the client cannot know my monotonic
// clock. I

// TODO inline the interval to make it time.Second internally? I should not
// allow time.Millisecond as this can not even be sent to a client via the
// x-ratelimit-reset header ;)

type TokenBucket struct {
	Max      uint64
	Interval time.Duration
	mu       sync.Mutex
	requests uint64
	reset    time.Time
}

// Limit shields the given handler from requests exceeding the rate of
// max requests per time interval. It does so using the token bucket algorithm.
// See https://en.wikipedia.org/wiki/Token_bucket
func (tb *TokenBucket) Limit(h http.Handler) http.Handler {
	maxHeader := strconv.FormatUint(tb.Max, 10)

	// TODO race on unsynchronized read/write of reset

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit", maxHeader)

		tb.mu.Lock()
		if c := time.Now(); c.After(tb.reset) {
			tb.requests = 0
			tb.reset = c.Add(tb.Interval)
		}
		w.Header().Set("x-ratelimit-reset", strconv.FormatInt(tb.reset.Unix(), 10))

		if tb.requests == tb.Max {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-used", maxHeader)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		tb.requests++
		// get copy of # requests (before releasing the lock )at the time the
		// rate limiter made its decision so the HTTP headers align with the
		// observed behavior by the client
		requests := tb.requests
		tb.mu.Unlock()

		w.Header().Set("x-ratelimit-remaining", strconv.FormatUint(tb.Max-requests, 10))
		w.Header().Set("x-ratelimit-used", strconv.FormatUint(requests, 10))
		h.ServeHTTP(w, r)
	})
}
