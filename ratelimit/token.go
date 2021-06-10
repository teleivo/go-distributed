// Package ratelimit implements a few different rate limiters aimed to protect API
// endpoints from excessive requests.
package ratelimit

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// TODO should I remove the monotonic clock from the reset and current time?
// if I send the reset time to a client, the client cannot know my monotonic
// clock. I

// TODO inline the interval to make it time.Second internally? I should not
// allow time.Millisecond as this can not even be sent via the reset header ;)

// TokenBucket shields the given handler from requests exceeding the rate of
// max requests per time interval. It does so using the token bucket algorithm.
// See https://en.wikipedia.org/wiki/Token_bucket
func TokenBucket(max uint64, interval time.Duration, h http.Handler) http.Handler {
	// TODO race on unsynchronized read/write of reset
	var reset time.Time
	tokens := max
	maxHeader := strconv.FormatUint(max, 10)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := time.Now(); c.After(reset) {
			atomic.SwapUint64(&tokens, max)
			reset = c.Add(interval)
		}
		w.Header().Set("x-ratelimit-reset", strconv.FormatInt(reset.Unix(), 10))
		w.Header().Set("x-ratelimit-limit", maxHeader)

		if atomic.LoadUint64(&tokens) == 0 {
			// if tokens == 0 {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-used", maxHeader)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		remaining := atomic.AddUint64(&tokens, ^uint64(0))
		// tokens--
		w.Header().Set("x-ratelimit-remaining", strconv.FormatUint(remaining, 10))
		w.Header().Set("x-ratelimit-used", strconv.FormatUint(max-remaining, 10))
		h.ServeHTTP(w, r)
	})
}
