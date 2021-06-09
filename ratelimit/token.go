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
	var reset time.Time
	tokens := max
	maxHeader := strconv.FormatUint(max, 10)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// c := time.Now()
		// fmt.Printf("%s, current %s\n", reset, c)
		// if c.After(reset) {
		// 	tokens = max
		// }
		// reset = c.Add(interval)

		if c := time.Now(); c.After(reset.Add(1 * time.Millisecond)) {
			// if c := time.Now(); c.After(reset) {
			tokens = max
			reset = c.Add(interval)
		}
		w.Header().Set("x-ratelimit-reset", strconv.FormatInt(reset.Unix(), 10))
		w.Header().Set("x-ratelimit-limit", maxHeader)

		if atomic.LoadUint64(&tokens) == 0 {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-used", maxHeader)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		atomic.AddUint64(&tokens, ^uint64(0))
		w.Header().Set("x-ratelimit-remaining", strconv.FormatUint(tokens, 10))
		w.Header().Set("x-ratelimit-used", strconv.FormatUint(max-tokens, 10))
		h.ServeHTTP(w, r)
	})
}
