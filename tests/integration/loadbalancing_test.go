package integration_tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/mouad-eh/wasseet/loadbalancer"
	"github.com/mouad-eh/wasseet/proxy"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinLoadBalancing(t *testing.T) {
	// start two backends
	var backend1RequestCount int
	var backend2RequestCount int
	backend1 := NewBackend("backend1", &backend1RequestCount)
	defer backend1.Close()

	backend2 := NewBackend("backend2", &backend2RequestCount)
	defer backend2.Close()

	// create a proxy config including the two backends
	backend1URL, _ := url.Parse(backend1.URL)
	backend2URL, _ := url.Parse(backend2.URL)

	backendUrls := []*url.URL{backend1URL, backend2URL}

	proxyConfig := &proxy.Config{
		Port: 0, // let OS assign an available port
		BackendGroups: []*proxy.BackendGroup{
			{
				Lb:      loadbalancer.NewRoundRobin(backendUrls),
				Servers: backendUrls,
			},
		},
		Rules: []*proxy.Rule{
			{
				Path:         "",
				BackendGroup: nil, // Will be set after
			},
		},
	}
	proxyConfig.Rules[0].BackendGroup = proxyConfig.BackendGroups[0]

	// start the proxy in a separate goroutine
	proxyServer := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	go func() {
		if err := proxyServer.Start(); err != nil && err != http.ErrServerClosed {
			t.Logf("Proxy server error: %v", err)
		}
	}()
	defer proxyServer.Stop()

	// wait for proxy to get its address
	var proxyURL string
	for i := 0; i < 50; i++ {
		if addr := proxyServer.GetAddr(); addr != "" {
			proxyURL = fmt.Sprintf("http://%s", addr)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NotEmpty(t, proxyURL, "proxy failed to start")

	// send requests to proxy and check if they are load balanced
	numRequests := 6
	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL + "/")
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Contains(t, []string{"backend1", "backend2"}, string(body))
	}

	require.Equal(t, 3, backend1RequestCount, "backend1 should receive 3 requests")
	require.Equal(t, 3, backend2RequestCount, "backend2 should receive 3 requests")
}

func NewBackend(name string, requestCount *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(name))
	}))
}
