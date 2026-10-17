package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("set INFRAI_API_KEY")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	api := &client{key: key, http: &http.Client{Timeout: 30 * time.Second}}
	mux := http.NewServeMux()
	mux.Handle("POST /digest", digestHandler(api))
	log.Printf("digest listener on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
