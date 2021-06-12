package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teleivo/go-distributed/ratelimit"
)

func TestTokenBucketRaceConditions(t *testing.T) {
	t.Run("ConcurrentRequestsCannotExceedLimit", func(t *testing.T) {
		var got uint64
		srv := httptest.NewServer(ratelimit.Limit(1, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&got, 1)
		})))
		defer srv.Close()

		var wg sync.WaitGroup
		for i := 0; i < 600; i++ {
			wg.Add(1)
			go func() {
				http.Get(srv.URL)
				wg.Done()
			}()
		}
		wg.Wait()

		if got != 1 {
			t.Errorf("Got %d, expected 1 call", got)
		}
	})
}
