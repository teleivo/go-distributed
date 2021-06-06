package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
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
		// TODO adjust logging prefix to a different timeformat to better
		// show elapsed seconds?
		// TODO print time between ticks to see why we sometimes get a 429
		case <-ticker.C:
			r, err := http.Get(*URL)
			if err != nil {
				log.Fatalf("Failed to request %q due to: %v", *URL, err)
			}
			log.Printf("%q responded with %d", *URL, r.StatusCode)
		case <-done:
			log.Println("Shutting down")
			return
		}
	}
}
