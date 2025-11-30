package proxy

import (
	"net/http"
)

// This interface is implemented by http.Client.
//
// The Proxy relies on it instead of relying directly on an http.Client
// in order to allow the usage of a mock implementation for testing.
type BackendClient interface {
	Do(ClientRequest) (*http.Response, error)
}

// HttpClient is an implementation of the BackendClient interface.
//
// It is a shallow wrapper around http.Client that does the conversion of ClientRequest to http.Request.
type HttpClient struct {
	Client *http.Client
}

func (h *HttpClient) Do(req ClientRequest) (*http.Response, error) {
	return h.Client.Do(req.Request)
}
