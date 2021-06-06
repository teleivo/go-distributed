package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/teleivo/go-distributed/ratelimit"
)

func main() {
	port := flag.Int("port", 8080, "Port at which the server listens for requests")
	rate := flag.Uint64("rate", 1, "Rate of requests/second to which the exposed endpoint will be limited to")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", ratelimit.TokenBucket(*rate, time.Second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello")
	})))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Exposing / on %s with rate limit of %d/seconds...", addr, *rate)
	log.Fatal(http.ListenAndServe(addr, mux))
}
