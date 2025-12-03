package loadbalancer

import (
	"net/url"
)

//go:generate moq -pkg mocks -out ../testutils/mocks/loadbalancer.go .  LoadBalancer

type LoadBalancer interface {
	Next() *url.URL
}
