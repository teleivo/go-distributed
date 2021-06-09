package ratelimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teleivo/go-distributed/ratelimit"
)

// TODO make tests resilient to an error in the implementation so that we do
// not hit the timeout of 10s?
// TODO try failing the tests through different errors in the impl and see how
// many fail and how
func TestTokenBucket(t *testing.T) {
	t.Run("AllowRequestsWithinLimit", func(t *testing.T) {
		var got uint64
		h := ratelimit.TokenBucket(1, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Request 2 denied
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}
		if got != 1 {
			t.Errorf("Got %d, expected 1 call", got)
		}
	})
	t.Run("ConcurrentRequestsCannotExceedLimit", func(t *testing.T) {
		var got uint64
		h := ratelimit.TokenBucket(1, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&got, 1)
		}))

		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				h.ServeHTTP(httptest.NewRecorder(), nil)
				wg.Done()
			}()
		}
		wg.Wait()

		if got != 1 {
			t.Errorf("Got %d, expected 1 call", got)
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
		httptest.NewServer(ratelimit.TokenBucket(1, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))

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

		if w := reset.Sub(time.Now()); w > interval {
			t.Fatalf("Would have to wait %s, which is longer than the rate limit interval %s", w, interval)
		}
		// TODO the issue is with the time.Now(); I should use the Time in the
		// Date header
		fmt.Println(rsp.Header.Get("Date")) // who sets the date header? do I need a fully fledged integration test for this one?
		time.Sleep(reset.Sub(time.Now()))
		fmt.Println("Test time after sleeping ", time.Now())

		// NOTE: converting X-Ratelimit-Reset into a time.Time gives a Time
		// without a monotonic clock. So even if time.Now().After(reset) ==
		// true in the test, it might not be in the actual rate limit handler.
		// That is because inside the handler the monotonic clocks of the
		// current time and the reset time will cause current.After(reset) to
		// become true. Since we cannot tell what the monotonic clocks are
		// inside of the actual implementation we have to retry several times
		// within a few hundred milliseconds to make sure we pass the reset time.
		for i := 0; i < 50; i++ {
			time.Sleep(10 * time.Millisecond)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, nil)

			rsp := rec.Result()
			if rsp.StatusCode != 200 {
				continue
			}

			rr, err := strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
			if err != nil {
				t.Fatalf("Failed to parse X-Ratelimit-Reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
			}
			newReset := time.Unix(rr, 0)
			if newReset.Sub(reset) != time.Second {
				fmt.Println(newReset.Sub(reset))
				t.Errorf("Got %s, expected %s added to the previous reset of %s, thus %s", newReset, interval, reset, reset.Add(interval))
			}
			return
		}
		t.Errorf("Rate limit was not reset within X-Ratelimit-Reset %s", reset)
	})
}
