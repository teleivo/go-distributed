package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	URL := flag.String("url", "http://localhost:8080/", "URL to request from")
	rate := flag.Uint64("rate", 1, "Rate limit of server in requests/second")
	flag.Parse()

	ticker := time.NewTicker(time.Duration(*rate) * time.Second)
	defer ticker.Stop()

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT)
	for {
		select {
		case <-ticker.C:
			rsp, err := http.Get(*URL)
			if err != nil {
				log.Fatalf("Failed to request %q due to: %v", *URL, err)
			}
			rr, err := strconv.ParseInt(rsp.Header.Get("X-Ratelimit-Reset"), 10, 0)
			if err != nil {
				log.Fatalf("Failed to parse X-Ratelimit-Reset %s due to: %v", rsp.Header.Get("X-Ratelimit-Reset"), err)
			}
			log.Printf("%q responded with %d, rate limit reset at %s", *URL, rsp.StatusCode, time.Unix(rr, 0))
		case <-done:
			log.Println("Shutting down")
			return
		}
	}
}
