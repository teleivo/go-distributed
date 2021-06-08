package ratelimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teleivo/go-distributed/ratelimit"
)

// TODO speed up tests with smaller interval?
// TODO make tests resilient to an error in the implementation so that we do
// not hit the timeout of 10s?
// TODO try failing the tests through different errors in the impl and see how
// many fail and how

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
		if got := rsp.Header.Get("X-Ratelimit-Remaining"); got != "0" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-remaining", got, 0)
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
	})
	t.Run("ResetTimeIsUnchangedByExceedingRequests", func(t *testing.T) {
		interval := time.Second
		h := ratelimit.TokenBucket(1, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		// Request within limit
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp := rec.Result()
		if got := rsp.StatusCode; got != 200 {
			t.Errorf("Got %d, expected status 200", got)
		}
		rr, err := strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse X-Ratelimit-Reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset1 := time.Unix(rr, 0)

		// Exceed rate limit
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}
		rr, err = strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse X-Ratelimit-Reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset2 := time.Unix(rr, 0)

		if !reset2.Equal(reset1) {
			t.Errorf("Got %s, expected it to be the same as the previous X-Ratelimit-Reset header %s", reset2, reset1)
		}

		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}
		rr, err = strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse X-Ratelimit-Reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset3 := time.Unix(rr, 0)

		if !reset3.Equal(reset1) {
			t.Errorf("Got %s, expected it to be the same as the previous X-Ratelimit-Reset header %s", reset2, reset1)
		}
	})
	t.Run("TokensRefreshAfterResetTimePassed", func(t *testing.T) {
		interval := time.Second
		h := ratelimit.TokenBucket(1, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, nil)
		rsp := rec.Result()
		if got := rsp.StatusCode; got != 200 {
			t.Errorf("Got %d, expected status 200", got)
		}

		// Exceed rate limit
		h.ServeHTTP(httptest.NewRecorder(), nil)
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}

		rr, err := strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse X-Ratelimit-Reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset := time.Unix(rr, 0)

		// NOTE: converting X-Ratelimit-Reset into a time.Time gives a Time
		// without a monotonic clock. So even if time.Now().After(reset) ==
		// true in the test, it might not be in the actual rate limit handler.
		// That is because inside the handler the monotonic clocks of the
		// current time and the reset time will cause current.After(reset) to
		// become true. Since we cannot tell what the monotonic clocks are
		// inside of the actual implementation we can retry a few times within
		// a few Milliseconds to make sure we pass the reset time.
		time.Sleep(reset.Sub(time.Now()))
		for {
			select {
			case <-time.Tick(time.Millisecond):
				// Request allowed again
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, nil)

				if rec.Result().StatusCode == 200 {
					// TODO test that the reset time has advanced by interval
					rr, err := strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
					if err != nil {
						t.Fatalf("Failed to parse X-Ratelimit-Reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
					}
					newReset := time.Unix(rr, 0)
					if newReset.Sub(reset) != time.Second {
						fmt.Println(newReset.Sub(reset))
						// t.Errorf("Got %s, expected it to be %s added to previous reset of %s, thus %s", newReset, interval, reset, reset.Add(interval))
					}
					return
				}
			case <-time.After(time.Second):
				t.Fatal("Timed out after 1s, waiting for rate limit to be lifted")
			}
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
