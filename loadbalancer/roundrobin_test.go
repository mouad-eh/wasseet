package loadbalancer_test

import (
	"net/url"
	"testing"

	"github.com/mouad-eh/wasseet/loadbalancer"
)

func TestRoundRobin(t *testing.T) {
	backends := []*url.URL{
		{Scheme: "http", Host: "backend1"},
		{Scheme: "http", Host: "backend2"},
		{Scheme: "http", Host: "backend3"},
	}
	rr := loadbalancer.NewRoundRobin(backends)
	for i := 0; i < len(backends)*2; i++ {
		backend := rr.Next()
		if backend != backends[i%len(backends)] {
			t.Errorf("Expected %s, got %s", backends[i%len(backends)], backend)
		}
	}
}
