package loadbalancer

import (
	"net/url"
)

type LoadBalancer interface {
	Next() *url.URL
}
