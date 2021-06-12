// Package rate implements a few different rate limiters aimed to protect API
// endpoints from excessive requests.
package rate

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
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
	tokens   uint64
	reset    time.Time
}

// Limit shields the given handler from requests exceeding the rate of
// max requests per time interval. It does so using the token bucket algorithm.
// See https://en.wikipedia.org/wiki/Token_bucket
func (tb *TokenBucket) Limit(h http.Handler) http.Handler {
	// TODO race on unsynchronized read/write of reset

	// TODO should this be done in a NewTokenBucket()?
	tb.tokens = tb.Max
	maxHeader := strconv.FormatUint(tb.Max, 10)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := time.Now(); c.After(tb.reset) {
			atomic.StoreUint64(&tb.tokens, tb.Max)
			tb.reset = c.Add(tb.Interval)
		}
		w.Header().Set("x-ratelimit-reset", strconv.FormatInt(tb.reset.Unix(), 10))
		w.Header().Set("x-ratelimit-limit", maxHeader)

		if atomic.LoadUint64(&tb.tokens) == 0 {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-used", maxHeader)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		remaining := atomic.AddUint64(&tb.tokens, ^uint64(0))
		w.Header().Set("x-ratelimit-remaining", strconv.FormatUint(remaining, 10))
		w.Header().Set("x-ratelimit-used", strconv.FormatUint(tb.Max-remaining, 10))
		h.ServeHTTP(w, r)
	})
}
