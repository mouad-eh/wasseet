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
	"github.com/mouad-eh/wasseet/proxy/config"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinLoadBalancing(t *testing.T) {
	// start multiple backend servers
	numBackends := 3
	backendServers := make([]*httptest.Server, numBackends)
	backendURLs := make([]*url.URL, numBackends)
	for i := 0; i < numBackends; i++ {
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("backend%d", i)))
		}))
		backendServers[i] = backendServer
		backendURLs[i], _ = url.Parse(backendServer.URL)
		defer backendServer.Close()
	}

	// create a proxy config including the started backend servers
	backendGroup := &config.BackendGroup{
		Lb:      loadbalancer.NewRoundRobin(backendURLs),
		Servers: backendURLs,
	}

	proxyConfig := &config.Config{
		Port: 0, // let OS assign an available port
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
			},
		},
	}

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
	numRequests := 8
	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL + "/")
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, fmt.Sprintf("backend%d", i%numBackends), string(body))
	}
}

func TestRoutingToMultipleBackendGroups(t *testing.T) {
	// start multiple backend servers
	numBackendGroups := 2
	backendServers := make([]*httptest.Server, numBackendGroups)
	backendURLs := make([]*url.URL, numBackendGroups)
	for i := 0; i < numBackendGroups; i++ {
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("backend%d", i)))
		}))
		backendServers[i] = backendServer
		backendURLs[i], _ = url.Parse(backendServer.URL)
		defer backendServer.Close()
	}

	// create backend groups where each group has one backend server
	backendGroups := make([]*config.BackendGroup, numBackendGroups)
	for i := 0; i < numBackendGroups; i++ {
		backendGroups[i] = &config.BackendGroup{
			Lb:      loadbalancer.NewRoundRobin([]*url.URL{backendURLs[i]}),
			Servers: []*url.URL{backendURLs[i]},
		}
	}

	// create routing rules for each backend group
	rules := make([]*config.Rule, numBackendGroups)
	for i := 0; i < numBackendGroups; i++ {
		rules[i] = &config.Rule{
			Path:         fmt.Sprintf("/api%d", i),
			BackendGroup: backendGroups[i],
		}
	}

	// create proxy config with routing rules for different paths
	proxyConfig := &config.Config{
		Port:          0,
		BackendGroups: backendGroups,
		Rules:         rules,
	}

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

	// test all valid routes + an invalid route
	for i := 0; i <= numBackendGroups; i++ {
		resp, err := http.Get(proxyURL + fmt.Sprintf("/api%d", i))
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		if i == numBackendGroups { // invalid route
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		} else { // valid route
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, fmt.Sprintf("backend%d", i), string(body))
		}
	}
}

func TestAddHeaderRequestOperation(t *testing.T) {
	testHeader := "X-Custom-Header"
	testHeaderValue := "test-header-value"

	// start a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// echo back the test header that was added by the proxy
		w.Write([]byte(r.Header.Get(testHeader)))
	}))
	backendURL, _ := url.Parse(backendServer.URL)
	defer backendServer.Close()

	// create proxy config with a AddHeaderRequest operation
	backendGroup := &config.BackendGroup{
		Lb:      loadbalancer.NewRoundRobin([]*url.URL{backendURL}),
		Servers: []*url.URL{backendURL},
	}
	proxyConfig := &config.Config{
		Port: 0,
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
				RequestOperations: []config.RequestOperation{
					&config.AddHeaderRequestOperation{
						Header: testHeader,
						Value:  testHeaderValue,
					},
				},
			},
		},
	}

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

	// send request to proxy and verify the custom header was added
	resp, err := http.Get(proxyURL + "/")
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, testHeaderValue, string(body))
}

func TestAddHeaderResponseOperation(t *testing.T) {
	// start a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	backendURL, _ := url.Parse(backendServer.URL)
	defer backendServer.Close()

	// create a backend group with one backend server
	backendGroup := &config.BackendGroup{
		Lb:      loadbalancer.NewRoundRobin([]*url.URL{backendURL}),
		Servers: []*url.URL{backendURL},
	}

	// create a rule with an AddHeaderResponseOperation
	testHeader := "X-Response-Header"
	testHeaderValue := "response-header-value"
	proxyConfig := &config.Config{
		Port: 0,
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
				ResponseOperations: []config.ResponseOperation{
					&config.AddHeaderResponseOperation{
						Header: testHeader,
						Value:  testHeaderValue,
					},
				},
			},
		},
	}

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

	// send request to proxy and verify the response header was added
	resp, err := http.Get(proxyURL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, testHeaderValue, resp.Header.Get(testHeader))
}
