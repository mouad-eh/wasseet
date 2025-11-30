package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/mouad-eh/wasseet/loadbalancer"
	"github.com/mouad-eh/wasseet/proxy"
)

func main() {
	backend1Port := 8081
	backend2Port := 8082

	go RunBackend(backend1Port, "backend 1")
	go RunBackend(backend2Port, "backend 2")

	backendUrls := []*url.URL{
		{Scheme: "http", Host: fmt.Sprintf("localhost:%d", backend1Port)},
		{Scheme: "http", Host: fmt.Sprintf("localhost:%d", backend2Port)},
	}

	proxyConfig := &proxy.Config{
		Port: 8080,
		BackendGroups: []*proxy.BackendGroup{
			{
				Lb:      loadbalancer.NewRoundRobin(backendUrls),
				Servers: backendUrls,
			},
		},
	}

	p := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	if err := p.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
}

func RunBackend(port int, name string) {
	mux := &http.ServeMux{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("Hello from %s", name)))
	})

	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start %s: %v", name, err)
	}
}
