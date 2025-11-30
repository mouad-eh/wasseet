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

func TestProxyHandler(t *testing.T) {
	backend := &url.URL{Scheme: "http", Host: "backend.io", Path: "/"}

	config := &proxy.Config{
		BackendGroups: []*proxy.BackendGroup{
			{
				Lb:      &NoopLoadBalancer{Backend: backend},
				Name:    "default",
				Servers: []*url.URL{backend},
			},
		},
	}

	beClient := NewMockBackendClient(backend)
	p := proxy.NewProxy(config, beClient)

	req := httptest.NewRequest("GET", "http://proxy.io", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, backend.String(), string(body))
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

	// // adapt client Request object so that it can be used by the handler
	// handlerCompatibleReq := httptest.NewRequest(req.Method, req.URL.String(), req.Body)
	// handlerCompatibleReq.Header = req.Header.Clone()

	serverReq := req.ToServerRequest()
	handlerCompatibleReq := serverReq.Request

	m.handler(w, handlerCompatibleReq)

	return w.Result(), nil
}
