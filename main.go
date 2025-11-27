package main

import (
	"log"
	"net/http"

	"github.com/mouad-eh/wasseet/proxy"
)

func main() {
	go runProxy()
	runBackend()
}

func runProxy() {
	p := proxy.NewProxy()
	if err := p.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
}

func runBackend() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("Failed to start backend: %v", err)
	}
}
