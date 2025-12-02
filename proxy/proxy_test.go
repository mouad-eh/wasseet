package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mouad-eh/wasseet/proxy"
	"github.com/stretchr/testify/require"
)

func TestRequestAndResponseForwarding(t *testing.T) {
	backend := &url.URL{Scheme: "http", Host: "backend.io", Path: "/"}

	config := &proxy.Config{
		BackendGroups: []*proxy.BackendGroup{
			{
				Lb:      &NoopLoadBalancer{Backend: backend},
				Name:    "default",
				Servers: []*url.URL{backend},
			},
		},
		Rules: []*proxy.Rule{
			{
				Path:         "",
				BackendGroup: nil, // Will be set after
			},
		},
	}
	config.Rules[0].BackendGroup = config.BackendGroups[0]

	beClient := &MockBackendClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(backend.String()))
		}),
	}
	p := proxy.NewProxy(config, beClient)

	req := httptest.NewRequest("GET", "http://proxy.io", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, backend.String(), string(body))
}

func TestRequestAndResponseMutation(t *testing.T) {
	backend := &url.URL{Scheme: "http", Host: "backend.io", Path: "/"}

	config := &proxy.Config{
		BackendGroups: []*proxy.BackendGroup{
			{
				Lb:      &NoopLoadBalancer{Backend: backend},
				Name:    "default",
				Servers: []*url.URL{backend},
			},
		},
		Rules: []*proxy.Rule{
			{
				BackendGroup: nil, // Will be set after
				RequestOperations: []proxy.RequestOperation{
					&proxy.AddHeaderRequestOperation{Header: "X-Custom-Request", Value: "request-value"},
				},
				ResponseOperations: []proxy.ResponseOperation{
					&proxy.AddHeaderResponseOperation{Header: "X-Custom-Response", Value: "response-value"},
				},
			},
		},
	}
	config.Rules[0].BackendGroup = config.BackendGroups[0]

	// Create a backend client that captures the request
	var capturedRequest *http.Request
	backendClient := &MockBackendClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedRequest = r
			w.WriteHeader(http.StatusOK)
		}),
	}

	p := proxy.NewProxy(config, backendClient)

	req := httptest.NewRequest("GET", "http://proxy.io", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.NotNil(t, capturedRequest)
	require.Equal(t, "request-value", capturedRequest.Header.Get("X-Custom-Request"))
	require.Equal(t, "response-value", resp.Header.Get("X-Custom-Response"))
}

type NoopLoadBalancer struct {
	Backend *url.URL
}

func (m *NoopLoadBalancer) Next() *url.URL {
	return m.Backend
}

// MockBackendClient is a mock implementation of BackendClient interface.
//
// It returns a response with 200 OK and the backend URL as the body.
type MockBackendClient struct {
	handler http.HandlerFunc
}

func NewMockBackendClient(backend *url.URL) *MockBackendClient {
	return &MockBackendClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(backend.String()))
		}),
	}
}

func (m *MockBackendClient) Do(req proxy.ClientRequest) (*http.Response, error) {
	w := httptest.NewRecorder()

	serverReq := req.ToServerRequest()
	handlerCompatibleReq := serverReq.Request

	m.handler(w, handlerCompatibleReq)

	return w.Result(), nil
}
