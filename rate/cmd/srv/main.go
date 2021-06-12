package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/teleivo/go-distributed/rate"
)

func main() {
	port := flag.Int("port", 8080, "Port at which the server listens for requests")
	max := flag.Uint64("max", 1, "Maximum number of requests/minute to which the exposed endpoint will be limited to")
	flag.Parse()

	mux := http.NewServeMux()
	tb := &rate.TokenBucket{Max: *max, Interval: time.Minute}
	mux.Handle("/", tb.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello")
	})))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Exposing / on %s with rate limit of %d/minute...", addr, *max)
	log.Fatal(http.ListenAndServe(addr, mux))
}
