package loadbalancer

import (
	"net/url"
)

type RoundRobin struct {
	backends []*url.URL
	current  int
}

func NewRoundRobin(backends []*url.URL) *RoundRobin {
	return &RoundRobin{
		backends: backends,
	}
}

func (r *RoundRobin) Next() *url.URL {
	backend := r.backends[r.current]
	r.current = (r.current + 1) % len(r.backends)
	return backend
}
