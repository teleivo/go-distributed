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
			t.Errorf("Got %d, expected status 429 when rate limit reached", got)
		}
		if got != 2 {
			t.Errorf("Got %d, expected 2 calls when rate limit reached", got)
		}
		if got := rsp.Header.Get("X-Ratelimit-Limit"); got != strconv.FormatUint(rate, 10) {
			t.Errorf("Got %s, expected %d for header x-ratelimit-limit when rate limit reached", got, rate)
		}
		if got := rsp.Header.Get("X-Ratelimit-Remaining"); got != "0" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-remaining when rate limit reached", got, 0)
		}
		if got := rsp.Header.Get("X-Ratelimit-Used"); got != "2" {
			t.Errorf("Got %s, expected %d for header x-ratelimit-used when rate limit reached", got, 2)
		}
	})
	t.Run("ResetTimeIsUnchangedByExceedingRequests", func(t *testing.T) {
		interval := time.Minute
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
			t.Fatalf("Failed to parse x-ratelimit-reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset1 := time.Unix(rr, 0)

		// necessary to show potential change in reset time without a delay an
		// implementation that would always set the reset
		// time.Now().Add(interval)
		// whould not fail the test
		time.Sleep(time.Second)

		// Exceed rate limit
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}
		rr, err = strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse x-ratelimit-reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset2 := time.Unix(rr, 0)

		if !reset2.Equal(reset1) {
			t.Errorf("Got %s, expected it to be the same as the previous x-ratelimit-reset header %s", reset2, reset1)
		}

		// necessary to show potential change in reset time without a delay an
		// implementation that would always set the reset
		// time.Now().Add(interval)
		// whould not fail the test
		time.Sleep(time.Second)

		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, nil)

		rsp = rec.Result()
		if got := rsp.StatusCode; got != 429 {
			t.Errorf("Got %d, expected status 429", got)
		}
		rr, err = strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse x-ratelimit-reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset3 := time.Unix(rr, 0)

		if !reset3.Equal(reset1) {
			t.Errorf("Got %s, expected it to be the same as the previous x-ratelimit-reset header %s", reset2, reset1)
		}
	})
	t.Run("TokensRefreshAfterResetTimePassed", func(t *testing.T) {
		interval := time.Second
		srv := httptest.NewServer(ratelimit.TokenBucket(1, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
		defer srv.Close()

		rsp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("Unexpected error while sending request: %v", err)
		}
		if got := rsp.StatusCode; got != 200 {
			t.Fatalf("Got %d, expected status 200", got)
		}

		// Exceed rate limit
		rsp, err = http.Get(srv.URL)
		if err != nil {
			t.Fatalf("Unexpected error while sending request: %v", err)
		}
		if got := rsp.StatusCode; got != 429 {
			t.Fatalf("Got %d, expected status 429", got)
		}
		resetHeader := rsp.Header.Get("X-Ratelimit-Reset")

		time.Sleep(interval)

		// Rate limit should be lifted
		rsp, err = http.Get(srv.URL)
		if err != nil {
			t.Fatalf("Unexpected error while sending request: %v", err)
		}
		if rsp.StatusCode != 200 {
			t.Fatalf("Got status %d, expected 200 since rate limit should be reset within %s", rsp.StatusCode, interval)
		}
		nextResetHeader := rsp.Header.Get("X-Ratelimit-Reset")

		rr, err := strconv.ParseInt(resetHeader, 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse x-ratelimit-reset header, got %s", resetHeader)
		}
		reset := time.Unix(rr, 0)
		rr, err = strconv.ParseInt(nextResetHeader, 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse next x-ratelimit-reset header, got %s", nextResetHeader)
		}
		nextReset := time.Unix(rr, 0)

		if nextReset.Sub(reset) != time.Second {
			fmt.Println(nextReset.Sub(reset))
			t.Errorf("Got %s, expected %s added to the previous reset of %s, thus %s", nextReset, interval, reset, reset.Add(interval))
		}
	})
	t.Run("DateAndResetHeaderDifferenceEqualsRatelimitInterval", func(t *testing.T) {
		interval := time.Second
		srv := httptest.NewServer(ratelimit.TokenBucket(1, interval, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
		defer srv.Close()

		rsp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("Unexpected error while sending request: %v", err)
		}
		if got := rsp.StatusCode; got != 200 {
			t.Fatalf("Got %d, expected status 200", got)
		}

		// Exceed rate limit
		rsp, err = http.Get(srv.URL)
		if err != nil {
			t.Fatalf("Unexpected error while sending request: %v", err)
		}
		if got := rsp.StatusCode; got != 429 {
			t.Fatalf("Got %d, expected status 429", got)
		}

		rr, err := strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
		if err != nil {
			t.Fatalf("Failed to parse x-ratelimit-reset header, got %s", rsp.Header.Get("X-Ratelimit-Reset"))
		}
		reset := time.Unix(rr, 0)
		date, err := time.Parse(time.RFC1123, rsp.Header.Get("Date"))
		if err != nil {
			t.Fatalf("Failed to parse Date header %s", rsp.Header.Get("Date"))
		}

		if got := reset.Sub(date); got != interval {
			t.Fatalf("Got %s, expected %s between the servers Date %s and rate limit reset %s", got, interval, date, reset)
		}
	})
}
