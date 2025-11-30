package proxy

import (
	"net/url"

	"github.com/mouad-eh/wasseet/loadbalancer"
)

type Config struct {
	Port          int
	BackendGroups []*BackendGroup
}

type BackendGroup struct {
	Name    string
	Lb      loadbalancer.LoadBalancer
	Servers []*url.URL
}
