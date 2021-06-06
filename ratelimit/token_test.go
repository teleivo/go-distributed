package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teleivo/go-distributed/ratelimit"
)

// TODO speed up tests with smaller interval?
// TODO write a test for the returned rate limit headers
// TODO write a test that shows that sending requests while the rate limiter is
// in the state of 429 will not prolong the wait necessary to get to 200

func TestTokenBucket(t *testing.T) {
	t.Run("AllowRequestsWithinLimit", func(t *testing.T) {
		var rate uint64 = 10
		interval := time.Second
		var got uint64
		h := ratelimit.TokenBucket(rate, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&got, 1)
		}))

		// TODO can I rewrite this in a more readable form to highight that I
		// want to send the requests within the given time frame? Maybe like in
		// the error case

		// Make rate number of calls at a rate of rate+1/interval
		var calls uint64
		s := time.Now()
		var e time.Time
		for range time.Tick(interval / time.Duration(rate+1)) {
			h.ServeHTTP(nil, nil)
			calls++
			if calls > rate-1 {
				e = time.Now()
				break
			}
		}
		// Ensure the calls are made within the interval
		// TODO is this necessary? Is it good enough to make rate number of
		// calls within interval/(rate*1.2); adding 20% as buffer?
		if d := e.Sub(s); d > interval {
			t.Errorf("Took %v, expected calls to be made within %v", d, interval)
		}
		if got != uint64(rate) {
			t.Errorf("Got %d, expected %d calls", got, rate)
		}
	})
	// TODO hard to read that I am making 11 requests but only expect 10 to
	// reach my wrapped handler
	t.Run("DenyRequestsExceedingRateLimit", func(t *testing.T) {
		var rate uint64 = 10
		interval := time.Second
		var got uint64
		h := ratelimit.TokenBucket(rate, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&got, 1)
		}))

		// Make rate number of calls at a rate of rate+1/interval
		var calls uint64
		var e time.Time
		start := time.Now()

		for ; calls < rate; calls++ {
			h.ServeHTTP(nil, nil)
		}
		e = time.Now()
		// Ensure the calls are made within the interval
		// TODO is this necessary? Is it good enough to make rate number of
		// calls within interval/(rate*1.2); adding 20% as buffer?
		if d := e.Sub(start); d > interval {
			t.Errorf("Took %v, expected calls to be made within %v", d, interval)
		}
		if got != uint64(rate) {
			t.Errorf("Got %d, expected %d calls", got, rate)
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, nil)
		if w.Result().StatusCode != 429 {
			t.Errorf("Got %d, expected status 429", w.Result().StatusCode)
		}

		w = httptest.NewRecorder()
		select {
		case e := <-time.After(interval - time.Now().Sub(start)):
			h.ServeHTTP(w, nil)
			if d := e.Sub(start); d < interval {
				t.Errorf("Waited %v before retrying, expected to wait until interval of %v elapsed", d, interval)
			}
			if w.Result().StatusCode != 200 {
				t.Errorf("Got %d, expected status 200 after waiting for %v before retrying", w.Result().StatusCode, e.Sub(start))
			}
		}
	})
}
