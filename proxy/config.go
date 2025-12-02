package proxy

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/mouad-eh/wasseet/loadbalancer"
)

type Config struct {
	Port          int
	BackendGroups []*BackendGroup
	// Ordering for rules is important.
	// To know the target backend group for a request, we start from the first rule and
	// move to the next one until we find a match or we reach the end of the list.
	Rules []*Rule
}

// GetFirstMatchingRule returns an error if there are no rules.
// We expect user to provide no rules if there is only one backend group, but
// in this case we should always have default rule that matches all requests.
func (c *Config) GetFirstMatchingRule(req ServerRequest) (*Rule, error) {
	if len(c.Rules) == 0 {
		return nil, fmt.Errorf("no rules provided, but there is more than one backend group")
	}
	for _, rule := range c.Rules {
		if rule.Match(req) {
			return rule, nil
		}
	}
	return nil, fmt.Errorf("no matching rule found for request: %v", req)
}

type BackendGroup struct {
	Name    string
	Lb      loadbalancer.LoadBalancer
	Servers []*url.URL
}

type Rule struct {
	Host               string
	Path               string
	BackendGroup       *BackendGroup
	RequestOperations  []RequestOperation
	ResponseOperations []ResponseOperation
}

func (r *Rule) Match(req ServerRequest) bool {
	if r.Host != "" && r.Host != req.Host {
		return false
	}
	if r.Path != "" && r.Path != req.URL.Path {
		return false
	}
	return true
}

func (r *Rule) ApplyRequestOperations(req ServerRequest) {
	for _, op := range r.RequestOperations {
		op.Apply(req)
	}
}

func (r *Rule) ApplyResponseOperations(resp *http.Response) {
	for _, op := range r.ResponseOperations {
		op.Apply(resp)
	}
}

type RequestOperation interface {
	Apply(req ServerRequest)
}

type ResponseOperation interface {
	Apply(resp *http.Response)
}

type AddHeaderRequestOperation struct {
	Header string
	Value  string
}

func (op *AddHeaderRequestOperation) Apply(req ServerRequest) {
	req.Header.Add(op.Header, op.Value)
}

type AddHeaderResponseOperation struct {
	Header string
	Value  string
}

func (op *AddHeaderResponseOperation) Apply(resp *http.Response) {
	resp.Header.Add(op.Header, op.Value)
}
