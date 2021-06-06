// Package ratelimit implements a few different rate limiters aimed to protect API
// endpoints from excessive requests.
package ratelimit

import (
	"net/http"
	"sync/atomic"
	"time"
)

// TODO how can I only allow interval of second, minute, hour?

// TokenBucket implements the token bucket algorithm shielding the given handler
// from requests exceeding the desired request limit/interval.
//
//
func TokenBucket(limit uint64, interval time.Duration, h http.Handler) http.Handler {
	tokens := limit
	var last time.Time
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := time.Now()
		if d := c.Sub(last); d >= interval {
			tokens = limit
		}
		// TODO this behavior should also be specified. Currently one has to
		// wait interval duration in between requests. Otherwise the timer
		// restarts and the client has to wait anew.
		last = c
		if atomic.LoadUint64(&tokens) == 0 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		atomic.AddUint64(&tokens, ^uint64(0))
		h.ServeHTTP(w, r)
	})
}
