package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teleivo/go-distributed/ratelimit"
)

// TODO write a test for the returned rate limit headers for error case
// TODO write a test that shows that sending requests while the rate limiter is
// in the state of 429 will not prolong the wait necessary to get to 200
// TODO speed up tests with smaller interval?

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
			h.ServeHTTP(httptest.NewRecorder(), nil)
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
	t.Run("RespondsWithRateLimitHeaders", func(t *testing.T) {
		var rate uint64 = 2
		interval := time.Minute
		var got uint64
		h := ratelimit.TokenBucket(rate, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&got, 1)
		}))

		// Request 1 allowed
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp := rec.Result()
		if got := rsp.StatusCode; got != 200 {
			t.Errorf("Got %d, expected status 200", got)
		}
		if got != 1 {
			t.Errorf("Got %d, expected 1 call", got)
		}
		if got := rsp.Header.Get("X-Ratelimit-Limit"); got != strconv.FormatUint(rate, 10) {
			t.Errorf("Got %s, expected %d for header x-ratelimit-limit", got, rate)
		}
		if got := rsp.Header.Get("X-Ratelimit-Remaining"); got != strconv.FormatUint(rate-1, 10) {
			t.Errorf("Got %s, expected %d for header x-ratelimit-remaining", got, rate-1)
		}
		if got := rsp.Header.Get("X-Ratelimit-Used"); got != "1" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-used", got, 1)
		}

		// Request 2 allowed
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 200 {
			t.Errorf("Got %d, expected status 200", got)
		}
		if got != 2 {
			t.Errorf("Got %d, expected 2 calls", got)
		}
		if got := rsp.Header.Get("X-Ratelimit-Limit"); got != strconv.FormatUint(rate, 10) {
			t.Errorf("Got %s, expected %d for header x-ratelimit-limit", got, rate)
		}
		if got := rsp.Header.Get("X-Ratelimit-Remaining"); got != strconv.FormatUint(rate-2, 10) {
			t.Errorf("Got %s, expected %d for header x-ratelimit-remaining", got, rate-1)
		}
		if got := rsp.Header.Get("X-Ratelimit-Used"); got != "2" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-used", got, 2)
		}

		// Request 3 denied
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}
		if got != 2 {
			t.Errorf("Got %d, expected 2 calls", got)
		}
		if got := rsp.Header.Get("X-Ratelimit-Limit"); got != strconv.FormatUint(rate, 10) {
			t.Errorf("Got %s, expected %d for header x-ratelimit-limit", got, rate)
		}
		if got := rsp.Header.Get("X-Ratelimit-Remaining"); got != "0" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-remaining", got, 0)
		}
		if got := rsp.Header.Get("X-Ratelimit-Used"); got != "2" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-used", got, 2)
		}
		// TODO test the timing
		// x-ratelimit-reset: 1622955974
		// TODO test that the first request will initiate the reset time
		// reset := time.Now().Unix()
		// test that the reset time stays the same for subsequent requests and
		// only changes once the time has passed
		// if got := rsp.Header.Get("X-Ratelimit-Reset"); got != strconv.FormatInt(time.Now().Unix(), 10) {
		// 	t.Errorf("Got %s, expected %d for header x-ratelimit-reset", got, 1)
		// }
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
			h.ServeHTTP(httptest.NewRecorder(), nil)
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
